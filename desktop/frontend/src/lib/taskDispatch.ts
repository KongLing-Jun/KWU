export type TaskDispatchStatus =
  | "dispatch_decision_required"
  | "dependency_not_satisfied"
  | "capacity_wait_required";

export interface TaskDispatchOutcome {
  status: TaskDispatchStatus;
  runningTaskRuns: string[];
  dependsOn: string[];
}

const statuses = new Set<TaskDispatchStatus>([
  "dispatch_decision_required",
  "dependency_not_satisfied",
  "capacity_wait_required",
]);

function stringList(value: unknown): string[] {
  return Array.isArray(value) ? value.filter((item): item is string => typeof item === "string" && item.trim() !== "") : [];
}

export function parseTaskDispatchOutcome(output?: string): TaskDispatchOutcome | null {
  if (!output?.trim()) return null;
  try {
    const value = JSON.parse(output) as Record<string, unknown>;
    if (!statuses.has(value.status as TaskDispatchStatus)) return null;
    return {
      status: value.status as TaskDispatchStatus,
      runningTaskRuns: stringList(value.running_task_runs),
      dependsOn: stringList(value.depends_on),
    };
  } catch {
    return null;
  }
}
