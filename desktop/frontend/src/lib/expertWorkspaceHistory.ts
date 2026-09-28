import type { ToolItem } from "../features/session/sessionItems";

export type RestoredExpertToolData = {
  args: string;
  output: string;
};

type ParallelTaskSpec = {
  member_name?: unknown;
  description?: unknown;
  prompt?: unknown;
};

function text(value: unknown): string {
  return typeof value === "string" ? value.trim() : "";
}

function parallelTaskSpecs(args: string): ParallelTaskSpec[] {
  if (!args.trim()) return [];
  try {
    const parsed = JSON.parse(args) as { tasks?: unknown };
    return Array.isArray(parsed?.tasks)
      ? parsed.tasks.filter((task): task is ParallelTaskSpec => Boolean(task && typeof task === "object" && !Array.isArray(task)))
      : [];
  } catch {
    return [];
  }
}

function parallelTaskHeaderIndex(line: string): number | null {
  if (line.length > 160) return null;
  const match = line.match(/\btask-(\d+)\b/i);
  if (!match) return null;
  const index = Number(match[1]);
  return Number.isInteger(index) && index > 0 ? index : null;
}

export function parallelTaskOutput(output: string, taskIndex: number): string {
  const lines = output.replace(/\r\n/g, "\n").split("\n");
  const wanted = taskIndex + 1;
  const start = lines.findIndex((line) => parallelTaskHeaderIndex(line) === wanted);
  if (start < 0) return "";
  let end = lines.length;
  for (let index = start + 1; index < lines.length; index += 1) {
    if (parallelTaskHeaderIndex(lines[index]) !== null) {
      end = index;
      break;
    }
  }
  return lines.slice(start + 1, end).join("\n").trim();
}

export function restoredParallelExpertItems(parent: ToolItem, data?: RestoredExpertToolData): ToolItem[] {
  if (parent.name !== "parallel_tasks" || !data) return [];
  return parallelTaskSpecs(data.args).map((task, index) => {
    const memberName = text(task.member_name);
    const description = text(task.description);
    const prompt = text(task.prompt);
    const output = parallelTaskOutput(data.output, index);
    const failed = /\[(?:FAILED|CANCELLED|SKIPPED)\]/i.test(output);
    return {
      kind: "tool",
      id: `${parent.id}/sub-${index + 1}`,
      parentId: parent.id,
      name: "task",
      args: JSON.stringify({
        ...(memberName ? { member_name: memberName } : {}),
        ...(description ? { description } : {}),
      }),
      readOnly: true,
      status: parent.status === "running" ? "running" : failed ? "error" : parent.status,
      output: output || undefined,
      error: failed ? output : undefined,
      subject: description || memberName || prompt || `task-${index + 1}`,
      summary: description || memberName || undefined,
    };
  });
}

export function restoreArchivedExpertTool(item: ToolItem, data?: RestoredExpertToolData): ToolItem {
  if (!data?.args) return item;
  return { ...item, args: data.args };
}

export function isRestorableExpertTool(item: ToolItem): boolean {
  return item.dataArchived === true
    && (item.name === "task" || item.name === "read_only_task" || item.name === "parallel_tasks");
}
