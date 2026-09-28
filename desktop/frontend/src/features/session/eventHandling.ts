import type { EventKind } from "../../lib/types";

export type EventHandlingOwner = "controller" | "task-tree" | "background";

export const EVENT_HANDLING_OWNER = {
  turn_started: "controller",
  reasoning: "controller",
  text: "controller",
  message: "controller",
  tool_dispatch: "controller",
  tool_result: "controller",
  tool_progress: "controller",
  usage: "controller",
  notice: "controller",
  phase: "controller",
  approval_request: "controller",
  ask_request: "controller",
  operational: "controller",
  turn_done: "controller",
  compaction_started: "controller",
  compaction_done: "controller",
  mcp_surface_ready: "background",
  retrying: "controller",
  steer: "controller",
  guardian_assessment: "controller",
  audit: "background",
  task_tree_created: "task-tree",
  task_node_updated: "task-tree",
  task_node_started: "task-tree",
  task_node_output: "task-tree",
  task_node_completed: "task-tree",
  task_node_failed: "task-tree",
  task_tree_completed: "task-tree",
  task_tree_summary: "task-tree",
  subagent_status_changed: "controller",
  final_candidate_rejected: "controller",
} as const satisfies Record<EventKind, EventHandlingOwner>;

export function eventHandlingOwner(kind: EventKind): EventHandlingOwner {
  return EVENT_HANDLING_OWNER[kind];
}
