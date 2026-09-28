import { asArray } from "../../lib/array";
import { stripHiddenSystemPrompt } from "../../lib/expertPrompt";
import { isReadOnlyTool, isSilentMCPAvailabilityNotice, type Item, type ToolItem } from "./sessionItems";
import { fileDiffFromWire, summarizeFileDiff } from "../../lib/tools";
import { displayAttachmentPathKey } from "../../lib/attachmentDisplay";
import { normalizeArtifactDeliveryV1 } from "../../lib/artifactDelivery";
import type { HistoryMessage, MemoryCitation, TaskLifecycle } from "../../lib/types";

const ARCHIVED_TOOL_ARG_LIMIT = 200;

function archivedToolArgs(args: string): string {
  return args && args.length > ARCHIVED_TOOL_ARG_LIMIT ? `${args.slice(0, ARCHIVED_TOOL_ARG_LIMIT)}…` : args;
}

function isCanonicalTodoTool(tool: ToolItem): boolean {
  return tool.name === "todo_write" && !tool.parentId && tool.status === "done" && !tool.error;
}

function latestCanonicalTodoToolIndex(items: Item[]): number {
  for (let index = items.length - 1; index >= 0; index -= 1) {
    const item = items[index];
    if (item.kind === "tool" && isCanonicalTodoTool(item)) return index;
  }
  return -1;
}

export function compactArchivedToolItems(items: Item[]): Item[] {
  const canonicalTodoIndex = latestCanonicalTodoToolIndex(items);
  return items.map((item, index) => {
    if (item.kind !== "tool" || item.status === "running") return item;
    const preserveArgs = index === canonicalTodoIndex;
    const nextArgs = preserveArgs ? item.args : archivedToolArgs(item.args);
    if (nextArgs === item.args && item.output === undefined && item.dataArchived === true) return item;
    return {
      ...item,
      args: nextArgs,
      output: undefined,
      dataArchived: true,
    };
  });
}

export function foldTaskLifecycleItems(items: Item[]): Item[] {
  const next = [...items];
  const latestTaskByRef = new Map<string, number>();
  for (let index = 0; index < next.length; index += 1) {
    const item = next[index];
    if (item.kind !== "tool") continue;
    if (item.name === "task" && item.subagentRef) {
      latestTaskByRef.set(item.subagentRef, index);
      continue;
    }
    if ((item.name !== "wait_task" && item.name !== "stop_task") || !item.taskLifecycleReceipts?.length) continue;
    const matchedRefs: string[] = [];
    for (const receipt of item.taskLifecycleReceipts) {
      const taskIndex = latestTaskByRef.get(receipt.subagentRef);
      if (taskIndex === undefined) continue;
      const task = next[taskIndex];
      if (task.kind !== "tool") continue;
      next[taskIndex] = { ...task, taskLifecycle: receipt };
      matchedRefs.push(receipt.subagentRef);
    }
    next[index] = {
      ...item,
      taskLifecycleRefs: matchedRefs.length === item.taskLifecycleReceipts.length ? matchedRefs : undefined,
    };
  }
  return projectVerifiedArtifactPaths(next);
}

function verifiedLifecycleArtifactPaths(receipt: TaskLifecycle): string[] {
  if (receipt.turnStatus !== "completed") return [];
  const observed = new Set(
    asArray(receipt.validations)
      .filter((validation) => validation.kind === "artifact_observed" && validation.success)
      .map((validation) => displayAttachmentPathKey(validation.path ?? ""))
      .filter(Boolean),
  );
  if (observed.size === 0) return [];
  const paths: string[] = [];
  const seen = new Set<string>();
  for (const artifact of asArray(receipt.artifacts)) {
    const path = artifact.path?.trim() ?? "";
    const key = displayAttachmentPathKey(path);
    if (!key || (hasHiddenDirectoryComponent(path) && artifact.source !== "writer") || !observed.has(key) || seen.has(key)) continue;
    seen.add(key);
    paths.push(path);
  }
  return paths;
}

function hasHiddenDirectoryComponent(path: string): boolean {
  const components = path.trim().replace(/\\/g, "/").split("/").filter(Boolean);
  return components.slice(0, -1).some((component) => component.length > 1 && component.startsWith(".") && component !== "..");
}

function nextUserIndex(items: Item[], start: number): number {
  for (let index = start + 1; index < items.length; index += 1) {
    if (items[index]?.kind === "user") return index;
  }
  return items.length;
}

function hasUserBefore(items: Item[], start: number): boolean {
  for (let index = start - 1; index >= 0; index -= 1) {
    if (items[index]?.kind === "user") return true;
  }
  return false;
}

function laterReceiptOwnsLifecycle(items: Item[], index: number, receipt: TaskLifecycle): boolean {
  const end = nextUserIndex(items, index);
  for (let candidateIndex = index + 1; candidateIndex < end; candidateIndex += 1) {
    const candidate = items[candidateIndex];
    if (candidate?.kind !== "tool") continue;
    if (candidate.taskLifecycleReceipts?.some((later) => later.subagentRef === receipt.subagentRef)) return true;
  }
  return false;
}

function lifecycleReceiptsAt(items: Item[], index: number): TaskLifecycle[] {
  const item = items[index];
  if (item?.kind !== "tool") return [];
  const receipts = [...asArray(item.taskLifecycleReceipts)];
  if (item.taskLifecycle && !laterReceiptOwnsLifecycle(items, index, item.taskLifecycle)) receipts.push(item.taskLifecycle);
  return receipts;
}

function deliveredToolArtifactPaths(item: ToolItem): string[] {
  if (item.name !== "deliver_files" || item.status !== "done" || item.error) return [];
  const paths: string[] = [];
  const seen = new Set<string>();
  for (const candidate of asArray(item.artifactPaths)) {
    if (typeof candidate !== "string") continue;
    const path = candidate.trim();
    const key = displayAttachmentPathKey(path);
    if (!key || seen.has(key)) continue;
    seen.add(key);
    paths.push(path);
  }
  return paths;
}

export function projectVerifiedArtifactPaths(items: Item[]): Item[] {
  const pathsByAssistant = new Map<number, string[]>();
  for (let sourceIndex = 0; sourceIndex < items.length; sourceIndex += 1) {
    if (!hasUserBefore(items, sourceIndex)) continue;
    const source = items[sourceIndex];
    const paths = [
      ...lifecycleReceiptsAt(items, sourceIndex).flatMap(verifiedLifecycleArtifactPaths),
      ...(source?.kind === "tool" ? deliveredToolArtifactPaths(source) : []),
    ];
    if (paths.length === 0) continue;
    const end = nextUserIndex(items, sourceIndex);
    let assistantIndex = -1;
    for (let index = end - 1; index > sourceIndex; index -= 1) {
      const candidate = items[index];
      if (candidate?.kind === "assistant" && candidate.finalAnswer === true) {
        assistantIndex = index;
        break;
      }
    }
    if (assistantIndex < 0) continue;
    const projected = pathsByAssistant.get(assistantIndex) ?? [];
    const seen = new Set(projected.map(displayAttachmentPathKey));
    for (const path of paths) {
      const key = displayAttachmentPathKey(path);
      if (!key || seen.has(key)) continue;
      seen.add(key);
      projected.push(path);
    }
    pathsByAssistant.set(assistantIndex, projected);
  }
  return items.map((item, index) => {
    if (item.kind !== "assistant") return item;
    const artifactDelivery = normalizeArtifactDeliveryV1(item.artifactDelivery);
    if (artifactDelivery) return { ...item, artifactDelivery };
    if (item.artifactDelivery !== undefined) {
      const { artifactPaths: _artifactPaths, ...contractItem } = item;
      return contractItem;
    }
    const artifactPaths = pathsByAssistant.get(index);
    if (artifactPaths) {
      const { artifactDelivery: _artifactDelivery, ...withoutArtifactDelivery } = item;
      return { ...withoutArtifactDelivery, artifactPaths };
    }
    if (!item.artifactPaths && item.artifactDelivery === undefined) return item;
    const { artifactDelivery: _artifactDelivery, artifactPaths: _artifactPaths, ...legacyItem } = item;
    return legacyItem;
  });
}

export function historyMessagesToItems(messages: HistoryMessage[], idPrefix: string, startSeq = 0): { items: Item[]; seq: number } {
  const resultByID = new Map<string, HistoryMessage>();
  for (const message of messages) {
    if (message.role === "tool" && message.toolCallId && !resultByID.has(message.toolCallId)) {
      resultByID.set(message.toolCallId, message);
    }
  }
  const positionalResults = positionalToolResults(messages);
  const consumedPositionalToolIndexes = new Set(Array.from(positionalResults.values(), (result) => result.index));

  const items: Item[] = [];
  let seq = startSeq;
  const consumedToolIDs = new Set<string>();
  for (let messageIndex = 0; messageIndex < messages.length; messageIndex += 1) {
    const message = messages[messageIndex];
    if (message.role === "system") continue;
    if (message.role === "phase") {
      const text = stripHiddenSystemPrompt(message.content);
      if (text.trim() !== "") {
        items.push({ kind: "phase", id: `${idPrefix}${seq}`, text });
        seq += 1;
      }
      continue;
    }
    if (message.role === "notice") {
      const text = stripHiddenSystemPrompt(message.content);
      if (text.trim() !== "" && !isSilentMCPAvailabilityNotice(text)) {
        items.push({ kind: "notice", id: `${idPrefix}${seq}`, level: message.level === "warn" ? "warn" : "info", text });
        seq += 1;
      }
      continue;
    }
    if (message.role === "compaction") {
      items.push({
        kind: "compaction",
        id: `${idPrefix}${seq}`,
        pending: Boolean(message.pending),
        trigger: message.trigger ?? "",
        messages: message.messages ?? 0,
        summary: message.summary ?? "",
        archive: message.archive ?? "",
      });
      seq += 1;
      continue;
    }
    if (message.role === "user") {
      const text = stripHiddenSystemPrompt(message.content);
      if (text.trim() === "") continue;
      items.push({ kind: "user", id: `${idPrefix}${seq}`, text, submitText: message.submitText, createdAt: message.createdAt });
      seq += 1;
      continue;
    }
    if (message.role === "assistant") {
      const text = stripHiddenSystemPrompt(message.content);
      const artifactDelivery = normalizeArtifactDeliveryV1(message.artifactDelivery) ?? message.artifactDelivery;
      const hasText = text.trim() !== "" || (message.reasoning ?? "").trim() !== "" || message.artifactDelivery !== undefined;
      if (hasText) {
        const memoryCitations = asArray<MemoryCitation>(message.memoryCitations);
        items.push({
          kind: "assistant",
          id: `${idPrefix}${seq}`,
          text,
          reasoning: message.reasoning ?? "",
          streaming: false,
          memoryCitations: memoryCitations.length > 0 ? memoryCitations : undefined,
          artifactDelivery,
          finalAnswer: message.finalAnswer === true || undefined,
          turn: message.turn,
        });
        seq += 1;
      }
      const toolCalls = message.toolCalls ?? [];
      for (let callIndex = 0; callIndex < toolCalls.length; callIndex += 1) {
        const toolCall = toolCalls[callIndex];
        const positionalResult = toolCall.id ? undefined : positionalResults.get(positionalToolResultKey(messageIndex, callIndex));
        const result = toolCall.id ? resultByID.get(toolCall.id) : positionalResult?.message;
        if (toolCall.id) consumedToolIDs.add(toolCall.id);
        const archived = Boolean(toolCall.argumentsArchived || result?.toolResultArchived);
        const output = result?.toolResultArchived ? undefined : result?.content ?? "";
        const error = result?.toolResultError || (output ? historyToolError(output) : undefined);
        const fileDiff = fileDiffFromWire(toolCall);
        const toolID = toolCall.id || `${idPrefix}tool${seq}`;
        items.push({
          kind: "tool",
          id: toolID,
          name: toolCall.name,
          args: toolCall.arguments ?? "",
          readOnly: isReadOnlyTool(toolCall.name),
          status: result ? (error ? "error" : "done") : "stopped",
          output,
          error,
          dataArchived: archived || undefined,
          subject: toolCall.subject,
          summary: summarizeFileDiff(fileDiff) || toolCall.summary,
          fileDiff,
          isShell: (toolCall.id || "").startsWith("shell-"),
          subagentRef: toolCall.subagentRef,
          taskLifecycle: toolCall.taskLifecycle,
          taskLifecycleRefs: toolCall.taskLifecycleRefs,
          taskLifecycleReceipts: toolCall.taskLifecycleReceipts,
          artifactPaths: toolCall.artifactPaths,
        });
        seq += 1;
      }
      continue;
    }
    if (message.role === "tool") {
      if ((message.toolCallId && consumedToolIDs.has(message.toolCallId)) || consumedPositionalToolIndexes.has(messageIndex)) continue;
      const output = message.toolResultArchived ? undefined : message.content;
      const error = message.toolResultError || (output ? historyToolError(output) : undefined);
      items.push({
        kind: "tool",
        id: message.toolCallId || `${idPrefix}tool${seq}`,
        name: message.toolName || "tool",
        args: "",
        readOnly: isReadOnlyTool(message.toolName || "tool"),
        status: error ? "error" : "done",
        output,
        error,
        dataArchived: message.toolResultArchived || undefined,
        isShell: (message.toolCallId || "").startsWith("shell-"),
      });
      seq += 1;
    }
  }
  return { items, seq };
}

function positionalToolResults(messages: HistoryMessage[]): Map<string, { message: HistoryMessage; index: number }> {
  const out = new Map<string, { message: HistoryMessage; index: number }>();
  const consumed = new Set<number>();
  for (let messageIndex = 0; messageIndex < messages.length; messageIndex += 1) {
    const message = messages[messageIndex];
    const toolCalls = message.role === "assistant" ? message.toolCalls ?? [] : [];
    if (toolCalls.length === 0) continue;
    let resultIndex = messageIndex + 1;
    for (let callIndex = 0; callIndex < toolCalls.length; callIndex += 1) {
      if (toolCalls[callIndex].id) continue;
      let matched = false;
      while (resultIndex < messages.length) {
        const candidate = messages[resultIndex];
        if (candidate.role !== "tool") break;
        const candidateIndex = resultIndex;
        resultIndex += 1;
        if (candidate.toolCallId || consumed.has(candidateIndex)) continue;
        consumed.add(candidateIndex);
        out.set(positionalToolResultKey(messageIndex, callIndex), { message: candidate, index: candidateIndex });
        matched = true;
        break;
      }
      if (!matched) break;
    }
  }
  return out;
}

function positionalToolResultKey(messageIndex: number, callIndex: number): string {
  return `${messageIndex}:${callIndex}`;
}

function historyToolError(output: string): string | undefined {
  const trimmed = output.trimStart();
  if (
    trimmed.startsWith("[error") ||
    trimmed.startsWith("Error:") ||
    trimmed.startsWith("error:") ||
    trimmed.startsWith("blocked:")
  ) return output;
  return undefined;
}
