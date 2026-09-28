import { stripHiddenSystemPrompt } from "../../lib/expertPrompt";
import { sealCurrentAssistant } from "./assistantBoundary";
import { applyTurnBilling, preserveTurnBilling } from "./turnBilling";
import { compactArchivedToolItems, foldTaskLifecycleItems, historyMessagesToItems } from "./historyProjection";
import { foregroundRunningFromRuntimeMeta, sameMeta } from "./sessionMeta";
import type { Item } from "./sessionItems";
import { initialState, type Action, type State } from "./sessionState";
import { historyMessagesFingerprint, mergeRevalidatedHistoryItems, mergeRuntimeTailItems } from "./historyRevalidation";
import { normalizeArtifactDeliveryV1 } from "../../lib/artifactDelivery";
import type { WireEvent } from "../../lib/types";
import { applyPromptReplyReceipt, beginPromptReply, failPromptReply, PROMPT_SEEN_EVENT_LIMIT, selectVisiblePrompt, type PromptProjectionState } from "./promptProjection";

export type EventProjector = (state: State, event: WireEvent) => State;

function normalizedUserText(text: string): string {
  return stripHiddenSystemPrompt(text).trim();
}

function projectTranscriptEvent(state: State, event: WireEvent, projectEvent: EventProjector): State {
  const journal = event.transcriptJournalId;
  const sequence = event.transcriptSequence;
  if (journal && sequence) {
    const sameJournal = journal === state.transcriptJournalId;
    if (state.transcriptJournalId && !sameJournal) return state;
    if (sameJournal && (sequence <= state.transcriptThrough || state.transcriptSeen[sequence])) return state;
    state = {
      ...state,
      transcriptJournalId: journal,
      transcriptThrough: sameJournal ? state.transcriptThrough : 0,
      transcriptSeen: { ...(sameJournal ? state.transcriptSeen : {}), [sequence]: true },
    };
  }
  if (event.transcriptUser) {
    const user = event.transcriptUser;
    // The accepted prompt may already be visible from the optimistic composer.
    // Keep this identity separate from the send/cancel marker, which unrelated
    // runtime events can confirm before the accepted prompt arrives.
    const pending = state.pendingTranscriptUser ?? state.pendingLocalUser;
    if (pending && normalizedUserText(pending.text) === normalizedUserText(user.content)
      && (!state.pendingTranscriptUser || user.createdAt === undefined || user.createdAt >= state.pendingTranscriptUser.createdAt)
      && state.items.some((item) => item.kind === "user" && item.id === pending.id)) {
      return { ...state, pendingLocalUser: undefined, pendingTranscriptUser: undefined, pendingUser: undefined };
    }
    state = sealCurrentAssistant(state);
    const projected = historyMessagesToItems([user], "tu", state.seq);
    return {
      ...state,
      items: [...state.items, ...projected.items],
      seq: projected.seq,
      pendingUser: pending ? state.pendingUser : undefined,
      pendingLocalUser: pending ? state.pendingLocalUser : undefined,
      surfaceIntent: "existing-session",
      running: true,
      cancellable: true,
    };
  }
  if (event.kind === "turn_done" && event.turnId && state.runtimeTurnId
    && (event.turnId !== state.runtimeTurnId || (!state.turnActive && state.running))) {
    return projectHistoricalTurnDone(state, event);
  }
  const next = projectEvent(state, event);
  return event.kind === "turn_done" ? { ...next, pendingTranscriptUser: undefined, turnStartAt: 0, turnTokens: 0 } : next;
}

function stateWithPrompts(state: State, prompts: PromptProjectionState): State {
  const visible = selectVisiblePrompt(prompts);
  return {
    ...state,
    prompts,
    approval: visible?.kind === "approval" ? visible.approval : undefined,
    ask: visible?.kind === "ask" ? visible.ask : undefined,
    pendingPrompt: Boolean(visible),
  };
}

function restoreLocalPromptReplies(restored: State, previous: State): State {
  let prompts = restored.prompts.prompts;
  for (const [id, prompt] of Object.entries(previous.prompts.prompts)) {
    const recovered = prompts[id];
    // Reply requests/receipts are local actions. Retain them only while the
    // journal has not advanced this prompt to a newer operational fact.
    if (recovered && (recovered.eventId !== prompt.eventId || recovered.sequence !== prompt.sequence)) continue;
    if (recovered === prompt) continue;
    if (prompts === restored.prompts.prompts) prompts = { ...prompts };
    prompts[id] = prompt;
  }
  if (prompts === restored.prompts.prompts) return restored;
  const seenEventOrder = [...new Set([...restored.prompts.seenEventOrder, ...previous.prompts.seenEventOrder])]
    .slice(-PROMPT_SEEN_EVENT_LIMIT);
  return stateWithPrompts(restored, {
    ...restored.prompts,
    prompts,
    nextArrival: Math.max(restored.prompts.nextArrival, previous.prompts.nextArrival),
    seenEventOrder,
    seenEventIds: Object.fromEntries(seenEventOrder.map((id) => [id, true as const])),
  });
}

function matchesPendingTranscriptUser(state: State, event: WireEvent): boolean {
  const pending = state.pendingTranscriptUser;
  const user = event.transcriptUser;
  if (!pending || !user || normalizedUserText(pending.text) !== normalizedUserText(user.content)) return false;
  if (typeof user.createdAt === "number") return user.createdAt >= pending.createdAt;
  return Boolean(pending.journalId && event.transcriptJournalId === pending.journalId
    && (event.transcriptSequence ?? 0) > pending.afterSequence);
}

function advanceTranscriptWatermark(state: State, through: number): State {
  const watermark = Math.max(state.transcriptThrough, through);
  const transcriptSeen = Object.fromEntries(Object.entries(state.transcriptSeen)
    .filter(([sequence]) => Number(sequence) > watermark));
  return { ...state, transcriptThrough: watermark, transcriptSeen };
}

function projectHistoricalTurnDone(state: State, event: WireEvent): State {
  const turnId = event.turnId?.trim() ?? "";
  if (
    event.kind !== "turn_done"
    || !turnId
    || event.interrupted === true
    || (event.err ?? "").trim() !== ""
    || event.artifactDelivery === undefined
  ) return state;

  let target = -1;
  for (let index = state.items.length - 1; index >= 0; index -= 1) {
    const item = state.items[index];
    if (item.kind === "assistant" && item.runtimeTurnId === turnId && !item.streaming) {
      target = index;
      break;
    }
  }
  if (target < 0) return state;

  const artifactDelivery = normalizeArtifactDeliveryV1(event.artifactDelivery) ?? event.artifactDelivery;
  const items = [...state.items];
  const item = items[target];
  if (item.kind !== "assistant" || item.artifactDelivery !== undefined) return state;
  items[target] = {
    ...item,
    finalAnswer: true,
    artifactDelivery,
  };
  return { ...state, items };
}

function normalizedModelRef(meta: State["meta"]): string {
  return meta?.modelRef?.trim() ?? "";
}

function stateWithMeta(state: State, meta: NonNullable<State["meta"]>, optimistic: boolean): State {
  const modelRef = normalizedModelRef(meta);
  const modelChanged = normalizedModelRef(state.meta) !== modelRef;
  if (!modelChanged && sameMeta(state.meta, meta)) return state;
  if (modelChanged) {
    return {
      ...state,
      meta,
      effort: undefined,
      composerSelection: {
        phase: "pending",
        seq: state.composerSelection.seq + 1,
        modelRef,
      },
      ...(optimistic ? { hydrateError: undefined } : {}),
    };
  }
  return { ...state, meta, ...(optimistic ? { hydrateError: undefined } : {}) };
}

export function reduceSession(state: State, action: Action, projectEvent: EventProjector): State {
  switch (action.type) {
    case "transcript_replay": {
      const pending = [...action.pending].sort((left, right) =>
        (left.transcriptSequence ?? Number.MAX_SAFE_INTEGER) - (right.transcriptSequence ?? Number.MAX_SAFE_INTEGER)
      );
      if (action.replay.after !== undefined) {
        // A completed runtime projection only needs events beyond its database
        // watermark. Preserve its item IDs, live pointers and local actions.
        if (action.replay.journalId !== state.transcriptJournalId) return state;
        let restored = state;
        for (const event of action.replay.events) restored = projectTranscriptEvent(restored, event, projectEvent);
        restored = advanceTranscriptWatermark(restored, action.replay.through);
        for (const event of pending) restored = projectTranscriptEvent(restored, event, projectEvent);
        return { ...restored, hydrateHistoryLoaded: true };
      }
      // Install the database snapshot and the events received during its read
      // in one reducer transition. Never concatenate two rendered transcripts.
      const optimistic = state.pendingTranscriptUser
        ? state.items.find((item) => item.kind === "user" && item.id === state.pendingTranscriptUser?.id)
        : undefined;
      const accepted = [...action.replay.events, ...pending].find((event) => matchesPendingTranscriptUser(state, event));
      let optimisticRestored = false;
      const restoreOptimistic = (current: State): State => {
        if (!optimistic || optimisticRestored) return current;
        optimisticRestored = true;
        return {
          ...current,
          items: [...current.items, optimistic],
          seq: Math.max(current.seq, state.seq),
          pendingUser: state.pendingUser,
          pendingLocalUser: state.pendingLocalUser,
          pendingTranscriptUser: state.pendingTranscriptUser,
          running: state.running,
          cancellable: state.cancellable,
          turnStartAt: state.turnStartAt,
          surfaceIntent: "existing-session",
        };
      };
      let restored: State = {
        ...initialState,
        meta: state.meta,
        effort: state.effort,
        composerSelection: state.composerSelection,
        balance: state.balance,
        context: state.context,
        jobs: state.jobs,
        checkpoints: state.checkpoints,
        messageAction: state.messageAction,
        backgroundJobs: state.backgroundJobs,
        sessionGen: state.sessionGen,
        prompts: { ...initialState.prompts, tabId: state.prompts.tabId },
        backendActivationPending: state.backendActivationPending,
        hydrating: state.hydrating,
        hydrateReason: state.hydrateReason,
        surfaceIntent: state.surfaceIntent,
      };
      restored = reduceSession(restored, { type: "history", messages: action.replay.messages }, projectEvent);
      for (const event of action.replay.events) {
        if (event === accepted) restored = restoreOptimistic(restored);
        // Snapshot events are already unique and ordered by the database.
        restored = projectTranscriptEvent(restored, { ...event, transcriptJournalId: undefined, transcriptSequence: undefined }, projectEvent);
      }
      restored = {
        ...restored,
        transcriptJournalId: action.replay.journalId,
        transcriptThrough: action.replay.through,
        transcriptSeen: {},
      };
      for (const event of pending) {
        if (event === accepted) restored = restoreOptimistic(restored);
        restored = projectTranscriptEvent(restored, event, projectEvent);
      }
      // An IPC read may finish before the backend accepts an optimistic send.
      // Its original bubble must remain until an accepted event identifies it.
      restored = restoreOptimistic(restored);
      if (!state.transcriptJournalId || state.transcriptJournalId === action.replay.journalId) {
        restored = { ...restored, items: preserveTurnBilling(state.items, restored.items) };
      }
      if (state.hydratePreview) {
        restored = { ...restored, items: mergeRevalidatedHistoryItems(state.hydratePreview.items, restored.items) };
      }
      return restoreLocalPromptReplies(restored, state);
    }
    case "user": {
      state = sealCurrentAssistant(state);
      const seq = action.seq !== undefined ? action.seq : state.seq;
      const id = `u${seq}`;
      const createdAt = Date.now();
      const afterSequence = Object.keys(state.transcriptSeen)
        .reduce((latest, sequence) => Math.max(latest, Number(sequence)), state.transcriptThrough);
      return {
        ...state,
        seq: seq + 1,
        items: [...state.items, { kind: "user", id, text: action.text, submitText: action.submitText, createdAt }],
        // The optimistic turn and its surface classification must publish together.
        surfaceIntent: state.surfaceIntent === "blank" ? "existing-session" : state.surfaceIntent,
        running: true,
        pendingPrompt: false,
        cancelRequested: false,
        cancellable: true,
        turnStartAt: createdAt,
        turnTokens: 0,
        turnTotalTokens: 0,
        turnCost: 0,
        pendingUser: action.text,
        pendingLocalUser: { id, text: action.text },
        pendingTranscriptUser: { id, text: action.text, createdAt, journalId: state.transcriptJournalId, afterSequence },
        discardTurn: false,
      };
    }
    case "unsend": return { ...state, items: removePendingLocalUser(state), pendingUser: undefined, pendingLocalUser: undefined, pendingTranscriptUser: undefined, discardTurn: true, running: false, pendingPrompt: false, cancelRequested: true, cancellable: false, approval: undefined, ask: undefined, live: undefined };
    case "cancel_requested": return { ...state, pendingPrompt: false, cancelRequested: true, approval: undefined, ask: undefined, cancellable: state.running || state.turnActive };
    case "send_failed": {
      if (state.pendingUser === undefined && !state.pendingLocalUser && !state.pendingTranscriptUser) return state;
      let index = -1;
      const pending = state.pendingLocalUser ?? state.pendingTranscriptUser;
      for (let itemIndex = state.items.length - 1; itemIndex >= 0; itemIndex -= 1) {
        const item = state.items[itemIndex];
        if (item.kind === "user" && ((pending && item.id === pending.id) || item.text === state.pendingUser)) {
          index = itemIndex;
          break;
        }
      }
      const items = index >= 0 ? state.items.map((item, itemIndex) => (itemIndex === index ? { ...item, failed: true } : item)) : state.items;
      const notice: Item = { kind: "notice", id: `n${state.seq}`, level: "warn", text: stripHiddenSystemPrompt(action.error) };
      return { ...state, pendingUser: undefined, pendingLocalUser: undefined, pendingTranscriptUser: undefined, running: false, turnActive: false, pendingPrompt: false, cancelRequested: false, cancellable: false, live: undefined, seq: state.seq + 1, items: [...items, notice] };
    }
    case "backend_status": {
      const pendingPrompt = Boolean(action.pendingPrompt);
      const backgroundJobs = Math.max(0, action.backgroundJobs ?? state.backgroundJobs ?? 0);
      const cancelRequested = Boolean(action.cancelRequested);
      const foregroundRunning = foregroundRunningFromRuntimeMeta({ running: action.running, pendingPrompt, backgroundJobs, cancellable: action.cancellable });
      const cancellable = foregroundRunning;
      const snapshotStartedAt = Math.max(0, action.activeTurnStartedAt ?? 0);
      const snapshotTokens = Math.max(0, action.activeTurnCompletionTokens ?? 0);
      let turnStartAt = state.turnStartAt;
      let turnTokens = state.turnTokens;
      if (foregroundRunning && snapshotStartedAt > 0) {
        if (turnStartAt === 0 || snapshotStartedAt > turnStartAt) {
          turnStartAt = snapshotStartedAt;
          turnTokens = snapshotTokens;
        } else if (snapshotStartedAt === turnStartAt) {
          turnTokens = Math.max(turnTokens, snapshotTokens);
        }
      }
      if (
        foregroundRunning === state.running &&
        pendingPrompt === state.pendingPrompt &&
        backgroundJobs === state.backgroundJobs &&
        cancelRequested === state.cancelRequested &&
        cancellable === state.cancellable &&
        turnStartAt === state.turnStartAt &&
        turnTokens === state.turnTokens &&
        (foregroundRunning || (state.turnStartAt === 0 && state.turnTokens === 0))
      ) return state;
      if (foregroundRunning) {
        return {
          ...state,
          running: true,
          turnActive: true,
          pendingPrompt,
          backgroundJobs,
          cancelRequested,
          cancellable,
          turnStartAt,
          turnTokens,
        };
      }
      const finalized = state.items.map((item) => {
        if (item.kind === "assistant" && state.live && item.id === state.live.id) return { ...item, text: stripHiddenSystemPrompt(state.live.text), reasoning: state.live.reasoning, streaming: false };
        if (item.kind === "assistant" && item.streaming) return { ...item, streaming: false };
        if (item.kind === "tool" && item.status === "running") return { ...item, status: "stopped" as const };
        return item;
      });
      const items = finalized.every((item, index) => item === state.items[index]) ? state.items : finalized;
      return { ...state, items, running: false, turnActive: false, pendingPrompt, backgroundJobs, cancelRequested, cancellable, live: undefined, currentAssistant: undefined, approval: undefined, ask: undefined, turnStartAt: 0, turnTokens: 0 };
    }
    case "meta": return stateWithMeta(state, action.meta, false);
    case "optimistic_meta": return stateWithMeta(state, action.meta, true);
    case "context": {
      const sessionTokens = typeof action.context.sessionTokens === "number"
        ? Math.max(0, action.context.sessionTokens)
        : state.sessionTokens;
      const snapshotStartedAt = Math.max(0, action.context.activeTurnStartedAt ?? 0);
      const snapshotTokens = Math.max(0, action.context.activeTurnCompletionTokens ?? 0);
      if (snapshotStartedAt === 0 || (state.turnStartAt > 0 && snapshotStartedAt < state.turnStartAt)) {
        return { ...state, context: action.context, sessionTokens };
      }
      if (snapshotStartedAt === state.turnStartAt) {
        return { ...state, context: action.context, sessionTokens, turnTokens: Math.max(state.turnTokens, snapshotTokens) };
      }
      return { ...state, context: action.context, sessionTokens, turnStartAt: snapshotStartedAt, turnTokens: snapshotTokens };
    }
    case "balance": return { ...state, balance: action.balance };
    case "turn_billing_results": {
      if (state.meta?.sessionPath !== action.sessionPath || state.sessionGen !== action.sessionGen || state.transcriptJournalId !== action.journalId) return state;
      const items = applyTurnBilling(state.items, action.results, action.scopeKey);
      return items === state.items ? state : { ...state, items };
    }
    case "turn_billing_settled": {
      let changed = false;
      const items = state.items.map((item) => {
        if (item.kind !== "assistant" || item.turn?.taskId !== action.taskId) return item;
        changed = true;
        return { ...item, turn: { ...item.turn, chargedCredits: action.chargedCredits } };
      });
      return changed ? { ...state, items } : state;
    }
    case "composer_selection_start": {
      if (action.seq <= state.composerSelection.seq) return state;
      const meta = action.meta ?? state.meta;
      return {
        ...state,
        meta,
        effort: action.invalidate ? undefined : state.effort,
        composerSelection: {
          phase: action.invalidate ? "pending" : state.composerSelection.phase,
          seq: action.seq,
          modelRef: action.modelRef.trim() || normalizedModelRef(meta) || state.composerSelection.modelRef,
          effortSnapshot: action.invalidate ? undefined : state.composerSelection.effortSnapshot,
        },
      };
    }
    case "composer_selection_ready": {
      const modelRef = action.modelRef.trim();
      if (action.seq !== state.composerSelection.seq || modelRef !== normalizedModelRef(action.meta)) return state;
      return {
        ...state,
        meta: action.meta,
        effort: action.effort,
        composerSelection: { phase: "ready", seq: action.seq, modelRef },
      };
    }
    case "composer_selection_degraded": {
      if (action.seq !== state.composerSelection.seq) return state;
      const meta = action.meta ?? state.meta;
      const modelRef = action.modelRef.trim() || normalizedModelRef(meta);
      if (action.meta && modelRef !== normalizedModelRef(action.meta)) return state;
      return {
        ...state,
        meta,
        effort: undefined,
        composerSelection: { phase: "degraded", seq: action.seq, modelRef },
      };
    }
    case "composer_effort_update_start": {
      const modelRef = action.modelRef.trim();
      if (
        action.seq <= state.composerSelection.seq
        || state.composerSelection.phase !== "ready"
        || modelRef !== state.composerSelection.modelRef
      ) return state;
      return {
        ...state,
        effort: action.effort,
        composerSelection: { phase: "updating-effort", seq: action.seq, modelRef },
      };
    }
    case "composer_effort_update_done": {
      if (
        action.seq !== state.composerSelection.seq
        || state.composerSelection.phase !== "updating-effort"
      ) return state;
      return {
        ...state,
        composerSelection: { ...state.composerSelection, phase: "ready" },
      };
    }
    case "composer_selection_restore": {
      if (action.seq !== state.composerSelection.seq) return state;
      return {
        ...state,
        meta: action.meta,
        effort: action.effort,
        composerSelection: { ...action.selection, seq: action.seq },
      };
    }
    case "jobs": return { ...state, jobs: action.jobs };
    case "checkpoints": return { ...state, checkpoints: action.checkpoints };
    case "hydrate_start": return {
      ...state,
      hydrating: true,
      surfaceIntent: action.surfaceIntent,
      hydrateReason: action.reason,
      hydrateError: undefined,
      hydrateHistoryLoaded: action.preserveHistory ? state.hydrateHistoryLoaded : false,
      hydratePlaceholderItems: action.placeholderItems?.length ? action.placeholderItems : undefined,
      hydratePlaceholderTabId: action.placeholderItems?.length ? action.placeholderTabId : undefined,
    };
    case "hydrate_done": return state.hydrating || state.hydrateReason || state.hydrateError || state.hydratePlaceholderItems || state.hydratePlaceholderTabId
      ? { ...state, hydrating: false, hydrateReason: undefined, hydrateError: undefined, hydratePlaceholderItems: undefined, hydratePlaceholderTabId: undefined }
      : state;
    case "hydrate_error": return { ...state, hydrating: false, hydrateReason: action.reason, hydrateError: action.error, hydrateHistoryLoaded: undefined, hydratePlaceholderItems: undefined, hydratePlaceholderTabId: undefined };
    case "backend_activation_start": return state.backendActivationPending ? state : { ...state, backendActivationPending: true };
    case "backend_activation_done": return state.backendActivationPending ? { ...state, backendActivationPending: false } : state;
    case "message_action_start": return { ...state, messageAction: action.action };
    case "message_action_done": return { ...state, messageAction: undefined };
    case "history": {
      const { items, seq } = historyMessagesToItems(action.messages, "h", state.seq);
      const projected = compactArchivedToolItems(foldTaskLifecycleItems(items));
      return {
        ...state,
        transcriptJournalId: undefined,
        transcriptThrough: 0,
        transcriptSeen: {},
        items: state.hydratePreview
          ? mergeRevalidatedHistoryItems(state.hydratePreview.items, projected)
          : preserveTurnBilling(state.items, projected),
        childItemsByRef: {},
        subagentStatusByRef: {},
        currentChildAssistantByRef: {},
        parallelItemsByCall: {},
        seq,
        historyFingerprint: historyMessagesFingerprint(action.messages),
        historyBefore: action.before ?? 0,
        historyGeneration: action.generation ?? "",
        historyHasMore: action.hasMore ?? false,
        historyLoadingOlder: false,
        hydrateHistoryLoaded: true,
        hydratePreview: undefined,
        hydratePlaceholderItems: undefined,
        hydratePlaceholderTabId: undefined,
      };
    }
    case "history_prepend": {
      const { items } = historyMessagesToItems(action.messages, `h${action.before}-`, 0);
      return {
        ...state,
        items: compactArchivedToolItems(foldTaskLifecycleItems([...items, ...state.items])),
        historyBefore: action.before,
        historyGeneration: action.generation,
        historyHasMore: action.hasMore,
        historyLoadingOlder: false,
        historyRevalidationSeq: state.historyRevalidationSeq + 1,
      };
    }
    case "history_older_loading": return state.historyLoadingOlder === action.loading
      ? state
      : { ...state, historyLoadingOlder: action.loading };
    case "history_revalidated": {
      const { items, seq } = historyMessagesToItems(action.messages, "h", 0);
      const projected = compactArchivedToolItems(foldTaskLifecycleItems(items));
      return {
        ...state,
        transcriptJournalId: undefined,
        transcriptThrough: 0,
        transcriptSeen: {},
        items: mergeRevalidatedHistoryItems(state.items, projected),
        seq,
        historyFingerprint: action.fingerprint,
        historyBefore: action.before ?? state.historyBefore,
        historyGeneration: action.generation ?? state.historyGeneration,
        historyHasMore: action.hasMore ?? state.historyHasMore,
        hydrateHistoryLoaded: true,
        hydratePreview: undefined,
        historyRevalidationSeq: state.historyRevalidationSeq + 1,
      };
    }
    case "history_with_runtime_tail": {
      const { items, seq } = historyMessagesToItems(action.messages, "h", 0);
      return {
        ...state,
        items: mergeRuntimeTailItems(compactArchivedToolItems(foldTaskLifecycleItems(items)), state.items),
        seq: Math.max(seq, state.seq),
        historyFingerprint: action.fingerprint,
        historyBefore: action.before ?? state.historyBefore,
        historyGeneration: action.generation ?? state.historyGeneration,
        historyHasMore: action.hasMore ?? state.historyHasMore,
        hydrateHistoryLoaded: true,
        hydratePreview: undefined,
        historyRevalidationSeq: state.historyRevalidationSeq + 1,
      };
    }
    case "local_notice": {
      const text = stripHiddenSystemPrompt(action.text);
      if (text.trim() === "") return { ...state, running: false, turnActive: false };
      return { ...state, running: false, turnActive: false, seq: state.seq + 1, items: [...state.items, { kind: "notice", id: `n${state.seq}`, level: action.level, text }] };
    }
    // Compatibility actions remain accepted, but lifecycle facts own clearing.
    case "clearApproval":
    case "clearAsk": return state;
    case "prompt_reply_started": return stateWithPrompts(state, beginPromptReply(state.prompts, action.promptId, action.input));
    case "prompt_reply_receipt": return stateWithPrompts(state, applyPromptReplyReceipt(state.prompts, action.receipt));
    case "prompt_reply_failed": return stateWithPrompts(state, failPromptReply(state.prompts, action.promptId, action.occurrenceId));
    case "reset": return { ...initialState, meta: state.meta, context: { ...state.context, used: 0, sessionTokens: 0 }, balance: state.balance, effort: state.effort, composerSelection: state.composerSelection, jobs: state.jobs, hydrating: state.hydrating, surfaceIntent: action.surfaceIntent ?? state.surfaceIntent, hydrateReason: state.hydrateReason, hydrateError: state.hydrateError, hydrateHistoryLoaded: state.hydrateHistoryLoaded, hydratePlaceholderItems: state.hydratePlaceholderItems, hydratePlaceholderTabId: state.hydratePlaceholderTabId, backendActivationPending: state.backendActivationPending, sessionGen: state.sessionGen + 1 };
    case "hydrate_reset": return {
      ...initialState,
      meta: state.meta,
      context: { ...state.context, used: 0, sessionTokens: 0 },
      balance: state.balance,
      effort: state.effort,
      composerSelection: state.composerSelection,
      jobs: state.jobs,
      prompts: state.prompts,
      approval: state.approval,
      ask: state.ask,
      running: state.running,
      turnActive: state.turnActive,
      pendingPrompt: state.pendingPrompt,
      backgroundJobs: state.backgroundJobs,
      cancelRequested: state.cancelRequested,
      cancellable: state.cancellable,
      turnStartAt: state.turnStartAt,
      turnTokens: state.turnTokens,
      hydrating: state.hydrating,
      surfaceIntent: state.surfaceIntent,
      hydrateReason: state.hydrateReason,
      hydrateError: state.hydrateError,
      hydrateHistoryLoaded: state.hydrateHistoryLoaded,
      hydratePreview: state.hydratePreview,
      hydratePlaceholderItems: state.hydratePlaceholderItems,
      hydratePlaceholderTabId: state.hydratePlaceholderTabId,
      backendActivationPending: state.backendActivationPending,
      sessionGen: state.sessionGen + 1,
    };
    case "event": {
      return projectTranscriptEvent(state, action.e, projectEvent);
    }
    case "historical_turn_done":
      return projectHistoricalTurnDone(state, action.e);
    case "stream_batch": {
      return action.events.reduce((current, event) => projectTranscriptEvent(current, event, projectEvent), state);
    }
    default: return state;
  }
}

function removePendingLocalUser(state: State): Item[] {
  const pending = state.pendingLocalUser ?? state.pendingTranscriptUser;
  if (!pending) return state.items;
  return state.items.filter((item) => !(item.kind === "user" && item.id === pending.id));
}
