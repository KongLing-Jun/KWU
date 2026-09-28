import type { Item, ToolItem } from "./sessionItems";
import { taskFailureDiagnostic } from "./taskFailureDiagnostic";

function hasTaskFailure(item: Item): item is ToolItem {
  return item.kind === "tool" && item.name === "task" && !item.parentId
    && (item.taskLifecycle?.turnStatus === "failed" || item.taskLifecycle?.turnStatus === "interrupted");
}

export function taskFailureCount(items: readonly Item[]): number {
  return items.reduce((count, item) => count + Number(hasTaskFailure(item)), 0);
}

// Keep the original transcript intact. A task inspector may replace a notice
// only when it owns that exact execution and retains the same diagnostic.
export function selectTaskNoticeItems(items: Item[]): Item[] {
  const owners = new Map<string, ToolItem | null>();
  const refCounts = new Map<string, number>();
  for (const item of items) {
    if (item.kind !== "tool" || item.name !== "task") continue;
    for (const ref of new Set([item.subagentRef, item.taskLifecycle?.subagentRef])) {
      if (ref) refCounts.set(ref, (refCounts.get(ref) ?? 0) + 1);
    }
    const runId = item.taskLifecycle?.taskRunId;
    if (!runId) continue;
    owners.set(runId, owners.has(runId) ? null : item);
  }

  const diagnostics = new Map<ToolItem, string | undefined>();
  const visible = items.filter((item) => {
    if (item.kind !== "notice" || item.parentId) return true;
    const match = /^background task failed: (\S+) \u2014 ([\s\S]+)$/.exec(item.text);
    if (!match) return true;
    const owner = owners.get(match[1]);
    if (!owner || !hasTaskFailure(owner)) return true;
    // Receipt projection currently targets the latest task by reference. A
    // reused reference cannot independently prove which run owns its error.
    const lifecycle = owner.taskLifecycle!;
    if (!owner.subagentRef || owner.subagentRef !== lifecycle.subagentRef
      || refCounts.get(owner.subagentRef) !== 1) return true;
    const error = lifecycle.error;
    if (!error) return true;
    if (error === match[2]) return false;
    if (!diagnostics.has(owner)) diagnostics.set(owner, taskFailureDiagnostic(error, lifecycle.turnStatus));
    return diagnostics.get(owner) !== match[2];
  });
  return visible.length === items.length ? items : visible;
}
