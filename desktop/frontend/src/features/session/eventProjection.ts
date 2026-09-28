import { asArray } from "../../lib/array";
import { sealCurrentAssistant } from "./assistantBoundary";
import { eventHandlingOwner } from "./eventHandling";
import { stripHiddenSystemPrompt } from "../../lib/expertPrompt";
import { formatGuardianAssessmentNotice } from "../../lib/guardianEvents";
import { compactArchivedToolItems, projectVerifiedArtifactPaths } from "./historyProjection";
import type { TaskLifecycle } from "../../lib/types";
import { isSilentMCPAvailabilityNotice, markNoticeForReveal, type Item, type ToolItem } from "./sessionItems";
import type { State } from "./sessionState";
import { applyTaskTreeEvent } from "../../lib/taskTreeState";
import { projectLegacyPrompt, projectOperationalPrompt, selectVisiblePrompt } from "./promptProjection";
import { projectOperational } from "./operationalProjection";
import { fileDiffFromWire, summarize, summarizeFileDiff } from "../../lib/tools";
import { normalizeArtifactDeliveryV1 } from "../../lib/artifactDelivery";
import type { MemoryCitation, WireEvent, WireUsage } from "../../lib/types";

function usageTotalTokens(usage?: WireUsage): number {
  if (!usage) return 0;
  if (usage.totalTokens > 0) return usage.totalTokens;
  const promptTokens = usage.promptTokens || usage.cacheHitTokens + usage.cacheMissTokens;
  return Math.max(0, promptTokens + usage.completionTokens);
}

function updatesContextGauge(usage?: WireUsage): boolean {
  const source = usage?.source?.trim();
  return !source || source === "executor";
}

function countsTowardCurrentTurn(state: State): boolean {
  return state.turnActive || state.running;
}

function eventParentId(event: WireEvent): string {
  return (event.parentId ?? "").trim();
}

function taskStartLine(output: string | undefined, prefix: string): string {
  const line = (output ?? "").split("\n").find((value) => value.trim().startsWith(prefix));
  return line?.trim().slice(prefix.length).trim() ?? "";
}

function taskStartList(output: string | undefined, prefix: string): string[] {
  const value = taskStartLine(output, prefix);
  return value ? value.split(",").map((entry) => entry.trim()).filter(Boolean) : [];
}

function taskLifecycleReceipts(output?: string): TaskLifecycle[] {
  const text = output?.trim() ?? "";
  if (!text) return [];
  try {
    const value = JSON.parse(text) as TaskLifecycle | TaskLifecycle[];
    const receipts = Array.isArray(value) ? value : [value];
    return receipts.filter((receipt) => Boolean(receipt?.subagentRef && receipt?.turnStatus));
  } catch {
    return [];
  }
}

function applyParentTaskLifecycle(items: Item[], controlIndex: number, receipts: TaskLifecycle[]): Item[] {
  if (receipts.length === 0) return items;
  const next = [...items];
  const control = next[controlIndex];
  if (control?.kind === "tool") {
    next[controlIndex] = { ...control, taskLifecycleRefs: receipts.map((receipt) => receipt.subagentRef), taskLifecycleReceipts: receipts };
  }
  for (const receipt of receipts) {
    for (let index = controlIndex - 1; index >= 0; index -= 1) {
      const candidate = next[index];
      if (candidate.kind === "tool" && candidate.name === "task" && candidate.subagentRef === receipt.subagentRef) {
        next[index] = { ...candidate, taskLifecycle: receipt };
        break;
      }
    }
  }
  return next;
}

function rootCandidateKey(event: Pick<WireEvent, "turnId" | "candidateId">): string {
  const turnId = event.turnId?.trim() ?? "";
  const candidateId = event.candidateId?.trim() ?? "";
  return turnId && candidateId ? `${turnId}\u0000${candidateId}` : "";
}

function itemMatchesRootCandidate(item: Item, event: Pick<WireEvent, "turnId" | "candidateId">): boolean {
  return item.kind === "assistant"
    && Boolean(event.turnId && event.candidateId)
    && item.runtimeTurnId === event.turnId
    && item.runtimeCandidateId === event.candidateId;
}

function ensureAssistant(state: State, event?: Pick<WireEvent, "turnId" | "candidateId">): { items: Item[]; id: string; seq: number } {
  if (event?.turnId && event.candidateId) {
    const existing = state.items.find((item) => itemMatchesRootCandidate(item, event));
    if (existing) return { items: state.items, id: existing.id, seq: state.seq };
  }
  if (state.currentAssistant) {
    const index = state.items.findIndex((item) => item.id === state.currentAssistant && item.kind === "assistant");
    const currentTurn = index >= 0 && state.items[index].kind === "assistant"
      ? (state.items[index] as Extract<Item, { kind: "assistant" }>).runtimeTurnId : undefined;
    if (index >= 0 && (!event?.turnId || !currentTurn || currentTurn === event.turnId)
      && !state.items.slice(index + 1).some((item) => item.kind === "user")) {
      const current = state.items[index];
      if (current.kind === "assistant" && event?.turnId && event.candidateId && !current.runtimeCandidateId) {
        const items = [...state.items];
        items[index] = { ...current, runtimeTurnId: event.turnId, runtimeCandidateId: event.candidateId };
        return { items, id: state.currentAssistant, seq: state.seq };
      }
      return { items: state.items, id: state.currentAssistant, seq: state.seq };
    }
  }
  const id = `a${state.seq}`;
  const item: Item = {
    kind: "assistant",
    id,
    text: "",
    reasoning: "",
    streaming: true,
    runtimeTurnId: event?.turnId,
    runtimeCandidateId: event?.candidateId,
  };
  return { items: [...state.items, item], id, seq: state.seq + 1 };
}

function ensureChildAssistant(state: State, subagentRef: string, parentId: string): { items: Item[]; id: string; seq: number } {
  const items = state.childItemsByRef[subagentRef] ?? [];
  const current = state.currentChildAssistantByRef[subagentRef];
  if (current && items.some((item) => item.kind === "assistant" && item.id === current)) {
    return { items, id: current, seq: state.seq };
  }
  const id = `child:${subagentRef}:a${state.seq}`;
  return {
    items: [...items, { kind: "assistant", id, parentId, text: "", reasoning: "", streaming: true }],
    id,
    seq: state.seq + 1,
  };
}

function clearCurrentChildAssistant(state: State, subagentRef: string): Record<string, string> {
  if (!state.currentChildAssistantByRef[subagentRef]) return state.currentChildAssistantByRef;
  const next = { ...state.currentChildAssistantByRef };
  delete next[subagentRef];
  return next;
}

function applyChildAssistantDelta(state: State, event: WireEvent, parentId: string, subagentRef: string): State {
  const { items, id, seq } = ensureChildAssistant(state, subagentRef, parentId);
  const delta = event.text ?? event.reasoning ?? "";
  const next = items.map((item) => {
    if (item.kind !== "assistant" || item.id !== id) return item;
    if (event.kind === "text") {
      return { ...item, text: item.text + delta, reasoningComplete: item.reasoning !== "" || item.reasoningComplete, streaming: true };
    }
    return { ...item, reasoning: item.reasoning + delta, reasoningComplete: false, streaming: true };
  });
  return {
    ...state,
    childItemsByRef: { ...state.childItemsByRef, [subagentRef]: next },
    currentChildAssistantByRef: { ...state.currentChildAssistantByRef, [subagentRef]: id },
    seq,
  };
}

function finalizeChildAssistant(state: State, event: WireEvent, parentId: string, subagentRef: string): State {
  const { items, id, seq } = ensureChildAssistant(state, subagentRef, parentId);
  const next = items.map((item) => {
    if (item.kind !== "assistant" || item.id !== id) return item;
    const memoryCitations = asArray<MemoryCitation>(event.memoryCitations ?? item.memoryCitations);
    return {
      ...item,
      text: stripHiddenSystemPrompt(event.text ?? item.text),
      reasoning: event.reasoning ?? item.reasoning,
      streaming: false,
      memoryCitations: memoryCitations.length > 0 ? memoryCitations : undefined,
    };
  });
  return {
    ...state,
    childItemsByRef: { ...state.childItemsByRef, [subagentRef]: next },
    currentChildAssistantByRef: clearCurrentChildAssistant(state, subagentRef),
    seq,
  };
}

function isParallelParent(state: State, parentId: string): boolean {
  return Boolean(state.parallelItemsByCall[parentId]) || state.items.some((item) => item.kind === "tool" && item.id === parentId && item.name === "parallel_tasks");
}

function applyChildToolDispatch(state: State, event: WireEvent, subagentRef: string): State {
  const tool = event.tool;
  if (!tool || tool.partial) return state;
  const items = state.childItemsByRef[subagentRef] ?? [];
  const id = tool.sourceToolId || tool.id || `child:${subagentRef}:tool${state.seq}`;
  const index = items.findIndex((item) => item.kind === "tool" && item.id === id);
  const args = tool.args ?? "";
  const fileDiff = fileDiffFromWire(tool);
  const projected: Item = {
    kind: "tool", id, name: tool.name, args, readOnly: tool.readOnly, status: "running",
    summary: summarizeFileDiff(fileDiff) || summarize(tool.name, args), fileDiff,
    isShell: id.startsWith("shell-"), parentId: tool.parentId, taskTreeId: tool.taskTreeId,
    nodeId: tool.nodeId, profile: tool.profile, subagentRef, sourceToolId: id,
  };
  const next = [...items];
  if (index >= 0) next[index] = { ...(next[index] as Extract<Item, { kind: "tool" }>), ...projected };
  else next.push(projected);
  return { ...state, seq: index >= 0 ? state.seq : state.seq + 1, childItemsByRef: { ...state.childItemsByRef, [subagentRef]: next } };
}

function applyChildToolResult(state: State, event: WireEvent, subagentRef: string): State {
  const tool = event.tool;
  if (!tool) return state;
  const items = [...(state.childItemsByRef[subagentRef] ?? [])];
  const id = tool.sourceToolId || tool.id;
  let index = id ? items.findIndex((item) => item.kind === "tool" && item.id === id) : -1;
  if (index < 0) {
    for (let i = items.length - 1; i >= 0; i -= 1) {
      const candidate = items[i];
      if (candidate.kind === "tool" && candidate.status === "running") { index = i; break; }
    }
  }
  if (index < 0) return state;
  const item = items[index];
  if (item.kind !== "tool") return state;
  items[index] = {
    ...item, status: tool.err ? "error" : "done", output: tool.output, error: tool.err,
    truncated: tool.truncated, durationMs: tool.durationMs,
    summary: tool.err ? undefined : item.summary || summarize(item.name, item.args, tool.output),
  };
  return { ...state, childItemsByRef: { ...state.childItemsByRef, [subagentRef]: compactArchivedToolItems(items) } };
}

function applyParallelToolEvent(state: State, event: WireEvent, parentId: string): State {
  const tool = event.tool;
  if (!tool) return state;
  const items = [...(state.parallelItemsByCall[parentId] ?? [])];
  const id = tool.id || `parallel:${parentId}:${state.seq}`;
  const index = items.findIndex((item) => item.id === id);
  if (event.kind === "tool_dispatch") {
    if (tool.partial) return state;
    const args = tool.args ?? "";
    const projected: Extract<Item, { kind: "tool" }> = {
      kind: "tool", id, name: tool.name, args, readOnly: tool.readOnly, status: "running",
      summary: summarize(tool.name, args), parentId,
    };
    if (index >= 0) items[index] = { ...items[index], ...projected };
    else items.push(projected);
    return { ...state, seq: index >= 0 ? state.seq : state.seq + 1, parallelItemsByCall: { ...state.parallelItemsByCall, [parentId]: items } };
  }
  if (event.kind === "tool_progress") {
    if (index < 0) return state;
    items[index] = { ...items[index], output: (items[index].output ?? "") + (tool.output ?? "") };
    return { ...state, parallelItemsByCall: { ...state.parallelItemsByCall, [parentId]: items } };
  }
  if (index < 0) return state;
  items[index] = {
    ...items[index], status: tool.err ? "error" : "done", output: tool.output, error: tool.err,
    truncated: tool.truncated, durationMs: tool.durationMs,
    summary: tool.err ? undefined : items[index].summary || summarize(items[index].name, items[index].args, tool.output),
  };
  return { ...state, parallelItemsByCall: { ...state.parallelItemsByCall, [parentId]: items } };
}

function flushPendingUser(state: State): State {
  if (state.pendingUser === undefined) return state;
  const lastItem = state.items[state.items.length - 1];
  if (lastItem?.kind === "user" && lastItem.text === state.pendingUser) {
    return { ...state, pendingUser: undefined };
  }
  const id = `u${state.seq}`;
  return {
    ...state,
    seq: state.seq + 1,
    items: [...state.items, { kind: "user", id, text: state.pendingUser, createdAt: Date.now() }],
    pendingUser: undefined,
    pendingLocalUser: { id, text: state.pendingUser },
  };
}

function dropEmptyCurrentAssistant(state: State): State {
  if (!state.currentAssistant) return state;
  const current = state.items.find((item) => item.kind === "assistant" && item.id === state.currentAssistant);
  if (current?.kind !== "assistant" || current.text || current.reasoning || state.live?.text || state.live?.reasoning) return state;
  return {
    ...state,
    items: state.items.filter((item) => item.id !== state.currentAssistant),
    currentAssistant: undefined,
    live: undefined,
  };
}

function eventConfirmsLocalUser(event: WireEvent): boolean {
  return event.kind !== "turn_started";
}

function turnDoneNoticeText(errorText: string, cancelRequested: boolean, interrupted: boolean): string {
  if (interrupted) return "本轮已中断";
  if (cancelRequested && errorText === "context canceled") return "本轮已中断";
  return errorText;
}

function retractCurrentParentCandidate(state: State, event: WireEvent): State {
  const identified = state.items.find((item) => itemMatchesRootCandidate(item, event));
  if (event.turnId && state.runtimeTurnId && event.turnId !== state.runtimeTurnId && !identified) return state;
  const currentID = identified?.id ?? state.rootFinalCandidateId ?? state.currentAssistant ?? state.live?.id;
  let candidateID = currentID;
  if (!candidateID) {
    for (let i = state.items.length - 1; i >= 0; i--) {
      const item = state.items[i];
      if (item.kind === "user") break;
      if (item.kind === "assistant") {
        candidateID = item.id;
        break;
      }
    }
  }
  const rejectedKey = rootCandidateKey(event);
  if (!candidateID && !rejectedKey) return state;
  return {
    ...state,
    items: candidateID ? state.items.filter((item) => item.id !== candidateID) : state.items,
    currentAssistant: state.currentAssistant === candidateID ? undefined : state.currentAssistant,
    live: state.live?.id === candidateID ? undefined : state.live,
    rootFinalCandidateId: state.rootFinalCandidateId === candidateID ? undefined : state.rootFinalCandidateId,
    rejectedRootCandidateKeys: rejectedKey
      ? { ...(state.rejectedRootCandidateKeys ?? {}), [rejectedKey]: true }
      : state.rejectedRootCandidateKeys,
  };
}

const persistedSubagentStatuses = new Set(["running", "completed", "failed", "interrupted"]);

function applySubagentStatusChanged(state: State, event: WireEvent): State {
  const ref = event.subagentRef?.trim() ?? "";
  const status = event.subagentStatus?.trim() ?? "";
  const current = state.subagentStatusByRef ?? {};
  if (!ref || !persistedSubagentStatuses.has(status) || current[ref] === status) return state;
  return { ...state, subagentStatusByRef: { ...current, [ref]: status } };
}

export function applyEvent(state: State, event: WireEvent): State {
  if (event.kind === "subagent_status_changed") return applySubagentStatusChanged(state, event);
  if (state.discardTurn) {
    if (event.kind === "turn_done") {
      return { ...state, discardTurn: false, running: false, turnActive: false, pendingPrompt: false, cancelRequested: false, cancellable: false, currentAssistant: undefined, rootFinalCandidateId: undefined, runtimeTurnId: undefined, rejectedRootCandidateKeys: {}, live: undefined };
    }
    return state;
  }
  const handlingOwner = eventHandlingOwner(event.kind);
  if (handlingOwner === "background") return state;
  const taskTrees = applyTaskTreeEvent(state.taskTrees, event);
  if (handlingOwner === "task-tree") {
    return taskTrees === state.taskTrees ? state : { ...state, taskTrees };
  }
  if (taskTrees !== state.taskTrees) state = { ...state, taskTrees };
  if (state.pendingUser !== undefined && event.kind !== "turn_done") state = flushPendingUser(state);
  if (state.pendingLocalUser && eventConfirmsLocalUser(event)) state = { ...state, pendingLocalUser: undefined };
  if (event.kind === "retrying") {
    return { ...state, retry: { attempt: event.retryAttempt ?? 0, max: event.retryMax ?? 0 } };
  }
  if (state.retry) state = { ...state, retry: undefined };

  switch (event.kind) {
    case "turn_started": {
      let current = state;
      if (current.pendingUser !== undefined) current = flushPendingUser(current);
      const turnId = event.turnId?.trim() || undefined;
      // Rebinding/replay can deliver the same start after reasoning has already
      // arrived. A turn identity starts once; replay must not clear its stream.
      if (turnId && current.runtimeTurnId === turnId) return current;
      if (turnId && current.items.some((item) => item.kind === "assistant" && item.runtimeTurnId === turnId)) return current;
      current = sealCurrentAssistant(current);
      const { items, id, seq } = ensureAssistant(current, event);
      const existingLive = turnId && current.live?.id === id
        && items.some((item) => item.kind === "assistant" && item.id === id && item.runtimeTurnId === turnId)
        ? current.live
        : undefined;
      return { ...current, items, currentAssistant: id, rootFinalCandidateId: undefined, runtimeTurnId: turnId, rejectedRootCandidateKeys: {}, seq, live: existingLive ?? { id, text: "", reasoning: "", reasoningComplete: false }, running: true, turnActive: true, pendingPrompt: false, cancelRequested: false, cancellable: true, turnStartAt: event.activeTurnStartedAt || Date.now(), turnTokens: existingLive ? current.turnTokens : 0, turnTotalTokens: existingLive ? current.turnTotalTokens : 0, turnCost: existingLive ? current.turnCost : 0 };
    }
    case "text":
    case "reasoning": {
      const parentId = eventParentId(event);
      const subagentRef = event.subagentRef?.trim() ?? "";
      if (subagentRef) return applyChildAssistantDelta(state, event, parentId, subagentRef);
      if (parentId) return state;
      if (event.turnId && state.runtimeTurnId
        && (event.turnId !== state.runtimeTurnId || (!state.turnActive && state.running))) return state;
      const replayedTerminal = state.items.find((item) => itemMatchesRootCandidate(item, event));
      if (replayedTerminal?.kind === "assistant" && !replayedTerminal.streaming && state.currentAssistant !== replayedTerminal.id) return state;
      const { items, id, seq } = ensureAssistant(state, event);
      const delta = event.text ?? event.reasoning ?? "";
      const base = state.live?.id === id ? state.live : { id, text: "", reasoning: "", reasoningComplete: false };
      const live = event.kind === "text"
        ? { ...base, text: base.text + delta, reasoningComplete: base.reasoning !== "" || base.reasoningComplete }
        : { ...base, reasoning: base.reasoning + delta, reasoningComplete: false };
      return { ...state, items, live, currentAssistant: id, rootFinalCandidateId: state.rootFinalCandidateId === id ? state.rootFinalCandidateId : undefined, seq };
    }
    case "message": {
      const parentId = eventParentId(event);
      const subagentRef = event.subagentRef?.trim() ?? "";
      if (subagentRef) return finalizeChildAssistant(state, event, parentId, subagentRef);
      if (parentId) return state;
      const candidateKey = rootCandidateKey(event);
      if (candidateKey && state.rejectedRootCandidateKeys?.[candidateKey]) return state;
      const identified = state.items.find((item) => itemMatchesRootCandidate(item, event));
      if (event.turnId && state.runtimeTurnId && event.turnId !== state.runtimeTurnId && !identified) return state;
      let terminalState = state;
      if (!identified && !candidateKey && state.rootFinalCandidateId) {
        terminalState = { ...state, currentAssistant: state.rootFinalCandidateId };
      }
      const { items, id, seq } = ensureAssistant(terminalState, event);
      const next = items.map((item) =>
        item.kind === "assistant" && item.id === id
          ? (() => {
              const memoryCitations = asArray<MemoryCitation>(event.memoryCitations ?? item.memoryCitations);
              return {
                ...item,
                text: stripHiddenSystemPrompt(event.text ?? state.live?.text ?? item.text),
                reasoning: event.reasoning ?? state.live?.reasoning ?? item.reasoning,
                streaming: false,
                memoryCitations: memoryCitations.length > 0 ? memoryCitations : undefined,
              };
            })()
          : item,
      );
      const currentTurn = !event.turnId || !state.runtimeTurnId || event.turnId === state.runtimeTurnId;
      return {
        ...state,
        items: next,
        live: state.live?.id === id ? undefined : state.live,
        currentAssistant: state.currentAssistant === id || terminalState.currentAssistant === id ? undefined : state.currentAssistant,
        rootFinalCandidateId: currentTurn ? id : state.rootFinalCandidateId,
        seq,
      };
    }
    case "tool_dispatch": {
      const tool = event.tool;
      if (!tool) return state;
      const subagentRef = tool.subagentRef?.trim() ?? "";
      if (subagentRef) return applyChildToolDispatch(state, event, subagentRef);
      if (tool.parentId) return isParallelParent(state, tool.parentId) ? applyParallelToolEvent(state, event, tool.parentId) : state;
      if (event.turnId && state.runtimeTurnId && event.turnId !== state.runtimeTurnId) return state;
      if (tool.partial) {
        // Only attributable live calls can create a preparation row. Argument
        // fragments are deliberately discarded until the full dispatch arrives.
        if (!tool.id?.trim() || !tool.name?.trim() || !state.turnActive || state.cancelRequested
          || !event.turnId || event.turnId !== state.runtimeTurnId
          || eventParentId(event) || event.subagentRef?.trim()
          || state.items.some((item) => item.kind === "tool" && item.id === tool.id)) return state;
        state = dropEmptyCurrentAssistant(state);
        return {
          ...state,
          seq: state.seq + 1,
          items: [...state.items, {
            kind: "tool", id: tool.id, name: tool.name, args: "", readOnly: tool.readOnly,
            status: "running", preparing: true, isShell: tool.id.startsWith("shell-"),
            taskTreeId: tool.taskTreeId, nodeId: tool.nodeId, profile: tool.profile,
          }],
        };
      }
      state = dropEmptyCurrentAssistant(state);
      const id = tool.id || `tool${state.seq}`;
      const index = state.items.findIndex((item) => item.kind === "tool" && item.id === id);
      if (index >= 0) {
        const next = [...state.items];
        const item = next[index];
        if (item.kind === "tool") {
          const args = tool.args ? tool.args : item.args;
          const fileDiff = fileDiffFromWire(tool);
          const summary = summarizeFileDiff(fileDiff) || summarize(tool.name, args) || (tool.name === item.name && args === item.args ? item.summary : undefined);
          next[index] = { ...item, preparing: undefined, name: tool.name, args, readOnly: tool.readOnly, taskTreeId: tool.taskTreeId ?? item.taskTreeId, nodeId: tool.nodeId ?? item.nodeId, profile: tool.profile ?? item.profile, summary, fileDiff };
        }
        return { ...state, items: next };
      }
      const args = tool.args ?? "";
      const fileDiff = fileDiffFromWire(tool);
      return {
        ...state,
        seq: state.seq + 1,
        items: [...state.items, { kind: "tool", id, name: tool.name, args, readOnly: tool.readOnly, status: "running", summary: summarizeFileDiff(fileDiff) || summarize(tool.name, args), fileDiff, isShell: id.startsWith("shell-"), parentId: tool.parentId, taskTreeId: tool.taskTreeId, nodeId: tool.nodeId, profile: tool.profile }],
      };
    }
    case "tool_result": {
      const tool = event.tool;
      if (!tool) return state;
      const subagentRef = tool.subagentRef?.trim() ?? "";
      if (subagentRef) return applyChildToolResult(state, event, subagentRef);
      if (tool.parentId) return isParallelParent(state, tool.parentId) ? applyParallelToolEvent(state, event, tool.parentId) : state;
      const next = [...state.items];
      let index = tool.id ? next.findIndex((item) => item.kind === "tool" && item.id === tool.id) : -1;
      if (index < 0 && event.turnId && state.runtimeTurnId && event.turnId !== state.runtimeTurnId) return state;
      if (index < 0) {
        for (let i = next.length - 1; i >= 0; i--) {
          const item = next[i];
          if (item.kind === "tool" && item.status === "running") { index = i; break; }
        }
      }
      if (index >= 0) {
        const item = next[index];
        if (item.kind === "tool") {
          const summary = tool.err ? undefined : item.summary || summarize(item.name, item.args, tool.output);
          next[index] = {
            ...item,
            status: tool.err ? "error" : "done",
            output: tool.output,
            error: tool.err,
            artifactPaths: !tool.err && item.name === "deliver_files" && tool.name === "deliver_files" && Array.isArray(tool.artifactPaths)
              ? [...tool.artifactPaths]
              : undefined,
            truncated: tool.truncated,
            durationMs: tool.durationMs,
            taskTreeId: tool.taskTreeId ?? item.taskTreeId,
            nodeId: tool.nodeId ?? item.nodeId,
            summary,
          };
          if (!tool.err && item.name === "task") {
            const ref = taskStartLine(tool.output, "Subagent reference:");
            if (ref && (tool.output ?? "").includes("Turn status: running")) {
              next[index] = {
                ...next[index] as ToolItem,
                subagentRef: ref,
                taskLifecycle: {
                  subagentRef: ref,
                  taskRunId: taskStartLine(tool.output, "Task run ID:") || undefined,
                  dependsOn: taskStartList(tool.output, "Dependencies:"),
                  turnStatus: "running",
                },
              };
            }
          } else if (!tool.err && (item.name === "wait_task" || item.name === "stop_task")) {
            return { ...state, items: compactArchivedToolItems(applyParentTaskLifecycle(next, index, taskLifecycleReceipts(tool.output))) };
          }
        }
      }
      return { ...state, items: compactArchivedToolItems(next) };
    }
    case "tool_progress": {
      const tool = event.tool;
      if (!tool?.id) return state;
      const subagentRef = tool.subagentRef?.trim() ?? "";
      if (subagentRef) {
        const items = [...(state.childItemsByRef[subagentRef] ?? [])];
        const id = tool.sourceToolId || tool.id;
        const index = items.findIndex((item) => item.kind === "tool" && item.id === id);
        if (index < 0) return state;
        const item = items[index];
        if (item.kind === "tool") items[index] = { ...item, output: (item.output ?? "") + (tool.output ?? "") };
        return { ...state, childItemsByRef: { ...state.childItemsByRef, [subagentRef]: items } };
      }
      if (tool.parentId) return isParallelParent(state, tool.parentId) ? applyParallelToolEvent(state, event, tool.parentId) : state;
      const index = state.items.findIndex((item) => item.kind === "tool" && item.id === tool.id);
      if (index < 0) return state;
      const next = [...state.items];
      const item = next[index];
      if (item.kind === "tool") next[index] = { ...item, output: (item.output ?? "") + (tool.output ?? "") };
      return { ...state, items: next };
    }
    case "usage": {
      if (event.turnId && state.runtimeTurnId && event.turnId !== state.runtimeTurnId) return state;
      if (!countsTowardCurrentTurn(state)) return state;
      const updateContextGauge = updatesContextGauge(event.usage);
      const used = event.usage && state.context.window && updateContextGauge ? event.usage.promptTokens : state.context.used;
      const snapshotStartedAt = Math.max(0, event.activeTurnStartedAt ?? 0);
      const snapshotTokens = Math.max(0, event.activeTurnCompletionTokens ?? 0);
      const turnTokens = snapshotStartedAt > 0 && snapshotStartedAt === state.turnStartAt
        ? Math.max(state.turnTokens, snapshotTokens)
        : state.turnTokens + (event.usage?.completionTokens ?? 0);
      const usageTokens = usageTotalTokens(event.usage);
      const turnTotalTokens = state.turnTotalTokens + usageTokens;
      const sessionTokens = state.sessionTokens + usageTokens;
      const usageCost = event.usage?.cost ?? event.usage?.costUsd ?? 0;
      const turnCost = state.turnCost + usageCost;
      const sessionCost = state.sessionCost + usageCost;
      const sessionCurrency = event.usage?.currency || state.sessionCurrency || "¥";
      const usage = updateContextGauge ? event.usage : state.usage ?? event.usage;
      return { ...state, usage, context: { ...state.context, used, sessionTokens }, turnTokens, turnTotalTokens, turnCost, sessionTokens, sessionCost, sessionCurrency };
    }
    case "notice": {
      const text = stripHiddenSystemPrompt(event.text ?? "");
      if (text.trim() === "" || isSilentMCPAvailabilityNotice(text)) {
        return { ...state, running: state.turnActive ? state.running : false };
      }
      return { ...state, running: state.turnActive ? state.running : false, seq: state.seq + 1, items: [...state.items, { kind: "notice", id: `n${state.seq}`, level: event.level ?? "info", text }] };
    }
    case "phase": {
      const text = stripHiddenSystemPrompt(event.text ?? "");
      if (text.trim() === "") return state;
      return { ...state, seq: state.seq + 1, items: [...state.items, { kind: "phase", id: `p${state.seq}`, text }] };
    }
    case "compaction_started":
      return { ...state, seq: state.seq + 1, items: [...state.items, { kind: "compaction", id: `c${state.seq}`, pending: true, trigger: event.compaction?.trigger ?? "", messages: 0, summary: "", archive: "" }] };
    case "compaction_done": {
      const compaction = event.compaction;
      const reverseIndex = [...state.items].reverse().findIndex((item) => item.kind === "compaction" && item.pending);
      const index = reverseIndex < 0 ? -1 : state.items.length - 1 - reverseIndex;
      if (!compaction?.summary) {
        const items = index < 0 ? state.items : state.items.filter((_, itemIndex) => itemIndex !== index);
        return { ...state, running: state.turnActive ? state.running : false, items };
      }
      const filled: Item = { kind: "compaction", id: index < 0 ? `c${state.seq}` : (state.items[index] as Extract<Item, { kind: "compaction" }>).id, pending: false, trigger: compaction.trigger ?? "", messages: compaction.messages ?? 0, summary: compaction.summary, archive: compaction.archive ?? "" };
      const items = index < 0 ? [...state.items, filled] : state.items.map((item, itemIndex) => (itemIndex === index ? filled : item));
      return { ...state, running: state.turnActive ? state.running : false, seq: state.seq + 1, items };
    }
    case "steer":
      return { ...state, seq: state.seq + 1, items: [...state.items, { kind: "notice", id: `s${state.seq}`, level: "info", text: stripHiddenSystemPrompt(`→ ${event.text ?? ""}`) }] };
    case "approval_request":
    case "ask_request": {
      if (state.cancelRequested) return state;
      const prompts = projectLegacyPrompt(state.prompts, event, event.tabId || state.prompts.tabId);
      if (prompts.prompts === state.prompts.prompts) {
        return prompts === state.prompts ? state : { ...state, prompts };
      }
      const visible = selectVisiblePrompt(prompts);
      return {
        ...state,
        prompts,
        approval: visible?.kind === "approval" ? visible.approval : undefined,
        ask: visible?.kind === "ask" ? visible.ask : undefined,
        pendingPrompt: Boolean(visible),
        running: true,
        turnActive: true,
        cancellable: true,
      };
    }
    case "operational": {
      if (!event.operational) return state;
      const operational = projectOperational(state.operational, event.operational);
      if (event.operational.aggregate.kind !== "prompt") {
        return operational === state.operational ? state : { ...state, operational };
      }
      const prompts = projectOperationalPrompt(state.prompts, event.operational);
      if (prompts === state.prompts && operational === state.operational) return state;
      const visible = selectVisiblePrompt(prompts);
      return {
        ...state,
        operational,
        prompts,
        approval: visible?.kind === "approval" ? visible.approval : undefined,
        ask: visible?.kind === "ask" ? visible.ask : undefined,
        pendingPrompt: Boolean(visible),
      };
    }
    case "guardian_assessment": {
      if (!event.guardian) return state;
      const level = event.guardian.outcome === "deny" ? "warn" : "info";
      return { ...state, seq: state.seq + 1, items: [...state.items, { kind: "notice", id: `g${state.seq}`, level, text: formatGuardianAssessmentNotice(event.guardian) }] };
    }
    case "final_candidate_rejected":
      return retractCurrentParentCandidate(state, event);
    case "turn_done": {
      if (state.pendingUser !== undefined) state = flushPendingUser(state);
      const errorText = stripHiddenSystemPrompt(event.err ?? "");
      const interrupted = event.interrupted === true || (state.cancelRequested && errorText === "context canceled");
      const visiblePrompt = selectVisiblePrompt(state.prompts);
      const keepPrompt = Boolean(visiblePrompt);
      const artifactDelivery = normalizeArtifactDeliveryV1(event.artifactDelivery) ?? event.artifactDelivery;
      const acceptedFinalID = errorText === "" && !interrupted && !keepPrompt
        ? state.rootFinalCandidateId
        : undefined;
      const finalized = state.items.map((item) => {
        if (item.kind === "assistant" && state.live && item.id === state.live.id) {
          return {
            ...item,
            text: stripHiddenSystemPrompt(state.live.text),
            reasoning: state.live.reasoning,
            streaming: false,
            finalAnswer: item.id === acceptedFinalID || undefined,
            ...(item.id === acceptedFinalID && artifactDelivery ? { artifactDelivery } : {}),
          };
        }
        if (item.kind === "assistant" && item.streaming) {
          return {
            ...item,
            streaming: false,
            finalAnswer: item.id === acceptedFinalID || undefined,
            ...(item.id === acceptedFinalID && artifactDelivery ? { artifactDelivery } : {}),
          };
        }
        if (item.kind === "assistant" && item.id === acceptedFinalID) {
          return {
            ...item,
            finalAnswer: true,
            ...(artifactDelivery ? { artifactDelivery } : {}),
          };
        }
        if (item.kind === "tool" && item.status === "running") return { ...item, status: "stopped" as const };
        return item;
      });
      if (event.turn) {
        for (let index = finalized.length - 1; index >= 0; index -= 1) {
          const item = finalized[index];
          if (item.kind === "user") break;
          if (item.kind !== "assistant") continue;
          finalized[index] = { ...item, turn: event.turn };
          break;
        }
      }
      const noticeText = turnDoneNoticeText(errorText, state.cancelRequested, event.interrupted === true);
      const notice: Extract<Item, { kind: "notice" }> | undefined = noticeText
        ? { kind: "notice", id: `e${state.seq}`, level: "warn", text: noticeText }
        : undefined;
      if (notice && interrupted) markNoticeForReveal(notice);
      const items: Item[] = projectVerifiedArtifactPaths(notice ? [...finalized, notice] : finalized);
      return { ...state, items, live: undefined, running: keepPrompt, turnActive: keepPrompt, pendingPrompt: keepPrompt, cancelRequested: false, cancellable: keepPrompt, currentAssistant: undefined, rootFinalCandidateId: acceptedFinalID, approval: visiblePrompt?.kind === "approval" ? visiblePrompt.approval : undefined, ask: visiblePrompt?.kind === "ask" ? visiblePrompt.ask : undefined, seq: state.seq + 1 };
    }
    default:
      return state;
  }
}
