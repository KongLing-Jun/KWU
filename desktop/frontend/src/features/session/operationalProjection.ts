import type { OperationalAggregateKind, WireOperationalEnvelope, WireOperationalLifecycle } from "../../lib/types";

export interface OperationalAggregateProjection {
  aggregateKind: OperationalAggregateKind;
  aggregateId: string;
  sequence: number;
  eventId: string;
  kind: string;
  lifecycle?: WireOperationalLifecycle;
  terminal: boolean;
}

export interface OperationalProjectionState {
  tabId?: string;
  aggregates: Record<string, OperationalAggregateProjection>;
  seenEventIds: string[];
}

const terminalSuffixes = new Set(["completed", "failed", "cancelled", "closed", "committed", "rejected", "exited", "resolved"]);

function isTerminal(envelope: WireOperationalEnvelope, suffix: string): boolean {
  if (envelope.aggregate.kind === "prompt") return suffix === "resolved" || suffix === "cancelled";
  return terminalSuffixes.has(suffix) && envelope.retryable !== true;
}

export function initialOperationalProjection(tabId?: string): OperationalProjectionState {
  return { tabId, aggregates: {}, seenEventIds: [] };
}

function payload(envelope: WireOperationalEnvelope): WireOperationalLifecycle | undefined {
  switch (envelope.aggregate.kind) {
    case "task": return envelope.task;
    case "model_call": return envelope.modelCall;
    case "tool_call": return envelope.toolCall;
    case "session": return envelope.session;
    case "dashboard_write": return envelope.dashboardWrite;
    case "plan": return envelope.plan;
    default: return undefined;
  }
}

export function projectOperational(state: OperationalProjectionState, envelope: WireOperationalEnvelope): OperationalProjectionState {
  if (!envelope.eventId || !envelope.aggregate?.kind || !envelope.aggregate.id || envelope.aggregate.sequence < 1) return state;
  if (state.tabId && envelope.scope.tabId && state.tabId !== envelope.scope.tabId) return state;
  if (state.seenEventIds.includes(envelope.eventId)) return state;
  const key = `${envelope.aggregate.kind}:${envelope.aggregate.id}`;
  const current = state.aggregates[key];
  if (current && (envelope.aggregate.sequence <= current.sequence || current.terminal)) return state;
  const suffix = envelope.kind.slice(envelope.kind.lastIndexOf(".") + 1);
  const next: OperationalAggregateProjection = {
    aggregateKind: envelope.aggregate.kind,
    aggregateId: envelope.aggregate.id,
    sequence: envelope.aggregate.sequence,
    eventId: envelope.eventId,
    kind: envelope.kind,
    lifecycle: payload(envelope),
    terminal: isTerminal(envelope, suffix),
  };
  return {
    tabId: state.tabId ?? envelope.scope.tabId,
    aggregates: { ...state.aggregates, [key]: next },
    seenEventIds: [...state.seenEventIds.slice(-511), envelope.eventId],
  };
}
