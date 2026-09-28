import type { TurnBillingResult } from "../../lib/types";
import type { Item } from "./sessionItems";

export function applyTurnBilling(items: Item[], results: TurnBillingResult[], scopeKey: string): Item[] {
  const byTask = new Map(results.map((result) => [result.taskId, result]));
  let changed = false;
  const next = items.map((item) => {
    if (item.kind !== "assistant" || !item.turn) return item;
    const result = byTask.get(item.turn.taskId);
    if (!result) return item;
    if (item.turn.billingScopeKey === scopeKey && item.turn.chargedCredits !== undefined) return item;
    const credits = result.status === "settled" && /^\d+(?:\.\d+)?$/.test(result.chargedCredits ?? "")
      ? result.chargedCredits : undefined;
    const status = result.status === "settled" && credits === undefined ? "error" : result.status;
    if (item.turn.billingScopeKey === scopeKey && item.turn.billingStatus === status && item.turn.chargedCredits === credits) return item;
    changed = true;
    return { ...item, turn: { ...item.turn, chargedCredits: credits, billingStatus: status, billingScopeKey: scopeKey } };
  });
  return changed ? next : items;
}

// Only carry facts for the same task. The view and query still validate the
// billing scope; a journal restore must not erase a newer local final result.
export function preserveTurnBilling(previous: Item[], next: Item[]): Item[] {
  const known = new Map(previous.flatMap((item) => item.kind === "assistant" && item.turn?.chargedCredits !== undefined
    ? [[item.turn.taskId, item.turn] as const] : []));
  return next.map((item) => {
    if (item.kind !== "assistant" || !item.turn || item.turn.chargedCredits !== undefined) return item;
    const turn = known.get(item.turn.taskId);
    return turn ? { ...item, turn: { ...item.turn, chargedCredits: turn.chargedCredits, billingStatus: turn.billingStatus, billingScopeKey: turn.billingScopeKey } } : item;
  });
}
