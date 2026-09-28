import type {
  PromptLifecycle,
  PromptReplyReceipt,
  QuestionAnswer,
  WireApproval,
  WireAsk,
  WireEvent,
  WireOperationalEnvelope,
} from "../../lib/types";

export type PromptReplyInput =
  | { kind: "approval"; allow: boolean; session: boolean; persist: boolean }
  | { kind: "ask"; answers: QuestionAnswer[] };

export type CanonicalPrompt = {
  promptId: string;
  tabId: string;
  sessionId?: string;
  turnId?: string;
  kind: "approval" | "ask";
  lifecycle: PromptLifecycle;
  sequence: number;
  arrival: number;
  eventId: string;
  causationId?: string;
  occurredAt?: string;
  occurrenceId?: string;
  retryable: boolean;
  approval?: WireApproval;
  ask?: WireAsk;
  lastReply?: PromptReplyInput;
  authority: "operational" | "legacy";
};

export type PromptProjectionState = {
  tabId: string;
  prompts: Record<string, CanonicalPrompt>;
  seenEventIds: Record<string, true>;
  seenEventOrder: string[];
  nextArrival: number;
};

export const PROMPT_SEEN_EVENT_LIMIT = 512;

export function initialPromptProjection(tabId = ""): PromptProjectionState {
  return { tabId, prompts: {}, seenEventIds: {}, seenEventOrder: [], nextArrival: 1 };
}

function rememberEvent(state: PromptProjectionState, eventId: string): PromptProjectionState {
  if (state.seenEventIds[eventId]) return state;
  const seenEventIds = { ...state.seenEventIds, [eventId]: true as const };
  const seenEventOrder = [...state.seenEventOrder, eventId];
  while (seenEventOrder.length > PROMPT_SEEN_EVENT_LIMIT) {
    const evicted = seenEventOrder.shift();
    if (evicted) delete seenEventIds[evicted];
  }
  return { ...state, seenEventIds, seenEventOrder };
}

function isTerminal(lifecycle: PromptLifecycle): boolean {
  return lifecycle === "resolved" || lifecycle === "cancelled";
}

function lifecycleForEnvelope(envelope: WireOperationalEnvelope): PromptLifecycle {
  if (envelope.prompt?.lifecycle) return envelope.prompt.lifecycle;
  switch (envelope.kind) {
    case "prompt.registered": return "registered";
    case "prompt.reply_requested": return "replying";
    case "prompt.resolved": return "resolved";
    case "prompt.reply_failed": return "reply_failed";
    case "prompt.cancelled": return "cancelled";
    default: return "pending";
  }
}

export function projectOperationalPrompt(state: PromptProjectionState, envelope: WireOperationalEnvelope): PromptProjectionState {
  if (envelope.schemaVersion !== 1 || envelope.aggregate.kind !== "prompt") return state;
  const promptId = envelope.aggregate.id.trim();
  const eventId = envelope.eventId.trim();
  const eventTabId = envelope.scope.tabId?.trim() ?? "";
  if (!promptId || !eventId || (state.tabId && eventTabId && state.tabId !== eventTabId)) return state;
  if (state.seenEventIds[eventId]) return state;

  const previous = state.prompts[promptId];
  const sequence = envelope.aggregate.sequence;
  if (previous?.authority === "operational" && sequence <= previous.sequence) {
    return rememberEvent(state, eventId);
  }
  const lifecycle = lifecycleForEnvelope(envelope);
  if (previous && isTerminal(previous.lifecycle) && !isTerminal(lifecycle)) {
    return rememberEvent(state, eventId);
  }
  const promptKind = envelope.prompt?.kind ?? previous?.kind;
  if (!promptKind) return state;
  const next: CanonicalPrompt = {
    ...previous,
    promptId,
    tabId: eventTabId || previous?.tabId || state.tabId,
    sessionId: envelope.scope.sessionId ?? previous?.sessionId,
    turnId: envelope.scope.turnId ?? previous?.turnId,
    kind: promptKind,
    lifecycle,
    sequence,
    arrival: state.nextArrival,
    eventId,
    causationId: envelope.causationId ?? previous?.causationId,
    occurredAt: envelope.occurredAt ?? previous?.occurredAt,
    occurrenceId: envelope.occurrenceId ?? previous?.occurrenceId,
    retryable: envelope.retryable ?? previous?.retryable ?? false,
    approval: envelope.prompt?.approval ?? previous?.approval,
    ask: envelope.prompt?.ask ?? previous?.ask,
    authority: "operational",
  };
  const remembered = rememberEvent(state, eventId);
  return {
    ...remembered,
    tabId: remembered.tabId || eventTabId,
    prompts: { ...remembered.prompts, [promptId]: next },
    nextArrival: state.nextArrival + 1,
  };
}

export function projectLegacyPrompt(state: PromptProjectionState, event: WireEvent, tabId: string): PromptProjectionState {
  const prompt = event.kind === "approval_request" ? event.approval : event.kind === "ask_request" ? event.ask : undefined;
  if (!prompt) return state;
  const sourceIdentity = event.eventId?.trim() || `${event.eventSequence ?? 0}`;
  const eventId = `legacy:${tabId}:${event.kind}:${prompt.id}:${sourceIdentity}`;
  const scoped = state.tabId ? state : { ...state, tabId };
  if (scoped.seenEventIds[eventId]) return scoped;
  const remembered = rememberEvent(scoped, eventId);
  const previous = remembered.prompts[prompt.id];
  // Compatibility transport ordering is deliberately separate from the
  // aggregate-local operational sequence. Once an operational fact exists,
  // legacy delivery can never replace or advance it.
  if (previous?.authority === "operational" || (previous && isTerminal(previous.lifecycle))) return remembered;
  const next: CanonicalPrompt = {
    ...previous,
    promptId: prompt.id,
    tabId,
    turnId: event.turnId ?? previous?.turnId,
    kind: event.kind === "approval_request" ? "approval" : "ask",
    lifecycle: "pending",
    sequence: 0,
    arrival: remembered.nextArrival,
    eventId,
    retryable: true,
    approval: event.kind === "approval_request" ? event.approval : previous?.approval,
    ask: event.kind === "ask_request" ? event.ask : previous?.ask,
    authority: "legacy",
  };
  return {
    ...remembered,
    prompts: { ...remembered.prompts, [prompt.id]: next },
    nextArrival: remembered.nextArrival + 1,
  };
}

export function beginPromptReply(state: PromptProjectionState, promptId: string, input: PromptReplyInput): PromptProjectionState {
  const previous = state.prompts[promptId];
  if (!previous || isTerminal(previous.lifecycle)) return state;
  return {
    ...state,
    prompts: {
      ...state.prompts,
      [promptId]: { ...previous, lifecycle: "replying", retryable: false, occurrenceId: undefined, lastReply: input },
    },
  };
}

export function applyPromptReplyReceipt(state: PromptProjectionState, receipt: PromptReplyReceipt): PromptProjectionState {
  const previous = state.prompts[receipt.promptId];
  if (!previous || isTerminal(previous.lifecycle)) return state;
  if (receipt.status === "accepted") return state;
  if (receipt.status === "already_resolved") {
    return {
      ...state,
      prompts: {
        ...state.prompts,
        // Only an operational terminal fact removes the prompt. Move a stale
        // reply out of the in-flight state while replay/resync catches up.
        [receipt.promptId]: { ...previous, lifecycle: "pending", retryable: false },
      },
    };
  }
  return {
    ...state,
    prompts: {
      ...state.prompts,
      [receipt.promptId]: {
        ...previous,
        lifecycle: receipt.status === "cancelled" ? "cancelled" : "reply_failed",
        occurrenceId: receipt.occurrenceId,
        retryable: receipt.retryable ?? receipt.status === "failed",
      },
    },
  };
}

export function failPromptReply(state: PromptProjectionState, promptId: string, occurrenceId?: string): PromptProjectionState {
  return applyPromptReplyReceipt(state, { promptId, status: "failed", occurrenceId, retryable: true });
}

export function selectVisiblePrompt(state: PromptProjectionState): CanonicalPrompt | undefined {
  return Object.values(state.prompts)
    .filter((prompt) => !isTerminal(prompt.lifecycle))
    // Aggregate sequence is intentionally scoped to one prompt and cannot be
    // compared across prompt IDs. Arrival orders the current visible shelf.
    .sort((a, b) => b.arrival - a.arrival)[0];
}
