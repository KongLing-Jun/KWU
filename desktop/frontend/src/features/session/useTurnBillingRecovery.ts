import { useEffect, useRef } from "react";
import { app, onTurnBillingSettled } from "../../lib/bridge";
import { cloudScopeKey, hasRecoverableCloudIdentity, useAccountStore } from "../../store/account";
import type { Action, State } from "./sessionState";
import type { TurnBillingResult } from "../../lib/types";

const batchSize = 64;

export function useTurnBillingRecovery(tabId: string | undefined, state: State, dispatch: (tabId: string, action: Action) => void) {
  const account = useAccountStore((store) => store.account);
  const scopeKey = account && hasRecoverableCloudIdentity(account) ? cloudScopeKey(account) : "";
  const latest = useRef({ tabId, state, scopeKey });
  latest.current = { tabId, state, scopeKey };
  const taskKey = JSON.stringify([...new Set(state.items.flatMap((item) =>
    item.kind === "assistant" && !item.streaming && item.turn?.billingSource === "subscription_pool"
      && (item.turn.chargedCredits === undefined || item.turn.billingScopeKey !== scopeKey)
      ? [item.turn.taskId] : []))]);
  const path = state.meta?.sessionPath ?? "";
  const journal = state.transcriptJournalId;
  const generation = state.sessionGen;

  useEffect(() => {
    const ids: string[] = JSON.parse(taskKey);
    if (!tabId || !path || !scopeKey || !ids.length || state.hydrating || !app.TurnBillingForTab) return;
    const identity = useAccountStore.getState().account;
    if (!identity || cloudScopeKey(identity) !== scopeKey) return;
    let disposed = false;
    let inFlight = false;
    let dirty = false;
    let offset = 0;
    let delay = 1000;
    let timer: ReturnType<typeof setTimeout> | undefined;
    const current = () => {
      const value = latest.current;
      const currentAccount = useAccountStore.getState().account;
      return !disposed && value.tabId === tabId && value.scopeKey === scopeKey
        && value.state.meta?.sessionPath === path && value.state.transcriptJournalId === journal
        && value.state.sessionGen === generation && !value.state.hydrating
        && currentAccount !== null && hasRecoverableCloudIdentity(currentAccount) && cloudScopeKey(currentAccount) === scopeKey;
    };
    const schedule = (ms: number) => {
      clearTimeout(timer);
      timer = setTimeout(() => { void read(); }, ms);
    };
    const read = async () => {
      if (!current() || document.hidden) return;
      if (inFlight) { dirty = true; return; }
      inFlight = true;
      const batch = ids.slice(offset, offset + batchSize);
      offset = offset + batchSize >= ids.length ? 0 : offset + batchSize;
      let results: TurnBillingResult[];
      try {
        results = await app.TurnBillingForTab!(tabId, {
          sessionPath: path, profileId: identity.profile_id, cloudUserId: identity.cloud_user_id!,
          bindingGeneration: identity.binding_generation!, taskIds: batch,
        });
      } catch {
        results = batch.map((taskId) => ({ taskId, status: "error" }));
      } finally {
        inFlight = false;
      }
      if (!current()) return;
      dispatch(tabId, { type: "turn_billing_results", results, scopeKey, sessionPath: path, journalId: journal, sessionGen: generation });
      // Notifications arriving during a read require a fresh SQLite snapshot.
      const nextDelay = dirty || offset !== 0 ? 0 : delay;
      dirty = false;
      if (offset === 0) delay = Math.min(delay * 2, 30000);
      schedule(nextDelay);
    };
    const wake = () => { if (current()) { delay = 1000; if (inFlight) dirty = true; else schedule(0); } };
    const off = onTurnBillingSettled(({ taskId }) => { if (ids.includes(taskId)) wake(); });
    window.addEventListener("focus", wake);
    document.addEventListener("visibilitychange", wake);
    schedule(0);
    return () => {
      disposed = true;
      clearTimeout(timer);
      off();
      window.removeEventListener("focus", wake);
      document.removeEventListener("visibilitychange", wake);
    };
  }, [tabId, path, journal, generation, scopeKey, taskKey, state.hydrating, dispatch]);
}
