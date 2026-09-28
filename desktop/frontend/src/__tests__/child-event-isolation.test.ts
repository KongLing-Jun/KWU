// Run: tsx src/__tests__/child-event-isolation.test.ts

import { readFileSync } from "node:fs";
import { reducer } from "../features/session/useController";
import { historyMessagesToItems } from "../features/session/historyProjection";
import { initialState } from "../features/session/sessionState";
import type { TaskLifecycle, WireEvent } from "../lib/types";

let passed = 0;
let failed = 0;

function ok(value: boolean, label: string) {
  if (value) {
    process.stdout.write(`  PASS  ${label}\n`);
    passed += 1;
  } else {
    process.stdout.write(`  FAIL  ${label}\n`);
    failed += 1;
  }
}

function eq(actual: unknown, expected: unknown, label: string) {
  ok(actual === expected, `${label}${actual === expected ? "" : `: expected ${JSON.stringify(expected)}, got ${JSON.stringify(actual)}`}`);
}

console.log("\nchild event isolation");

let taskState = reducer(initialState, { type: "event", e: { kind: "turn_started" } });
taskState = reducer(taskState, {
  type: "event",
  e: { kind: "tool_dispatch", tool: { id: "task-1", name: "task", args: "{}", readOnly: false } },
});
eq(taskState.items.length, 1, "tool-only parent turn removes its empty assistant placeholder");
eq(taskState.items[0]?.kind, "tool", "parent transcript retains the outer task card");

taskState = reducer(taskState, {
  type: "event",
  e: { kind: "reasoning", reasoning: "child thinks", parentId: "task-1", subagentRef: "child-1" },
});
taskState = reducer(taskState, {
  type: "event",
  e: {
    kind: "tool_dispatch",
    tool: {
      id: "task-1/tool-raw", sourceToolId: "tool-raw", name: "bash", args: "{}", readOnly: false,
      parentId: "task-1", subagentRef: "child-1",
    },
  },
});
taskState = reducer(taskState, {
  type: "event",
  e: {
    kind: "tool_result",
    tool: {
      id: "task-1/tool-raw", sourceToolId: "tool-raw", name: "bash", output: "ok", readOnly: false,
      parentId: "task-1", subagentRef: "child-1",
    },
  },
});
taskState = reducer(taskState, {
  type: "event",
  e: { kind: "message", text: "child final", parentId: "task-1", subagentRef: "child-1" },
});
eq(taskState.items.length, 1, "child first event and final never enter parent items");
eq(taskState.childItemsByRef["child-1"]?.length, 2, "child assistant and tool are buffered by stable ref");
const childTool = taskState.childItemsByRef["child-1"]?.find((item) => item.kind === "tool");
eq(childTool?.id, "tool-raw", "child buffer uses the raw child tool id");
eq(childTool?.kind === "tool" ? childTool.status : undefined, "done", "child result matches its raw tool id");
eq(childTool?.kind === "tool" ? childTool.dataArchived : undefined, true, "completed child tool follows the existing archive policy");

let spanningState = reducer(initialState, {
  type: "event",
  e: { kind: "reasoning", reasoning: "before parent turn", parentId: "task-span", subagentRef: "child-span" },
});
spanningState = reducer(spanningState, { type: "user", text: "new parent turn", seq: spanningState.seq });
spanningState = reducer(spanningState, {
  type: "event",
  e: { kind: "message", text: "same child turn", parentId: "task-span", subagentRef: "child-span" },
});
eq(spanningState.childItemsByRef["child-span"]?.filter((item) => item.kind === "assistant").length, 1, "parent turns do not split a running child assistant");

const legacyParented = reducer(taskState, {
  type: "event",
  e: { kind: "message", text: "legacy child", parentId: "task-1" },
});
eq(legacyParented.items.length, 1, "unidentified parented assistant fails closed instead of polluting main items");

let parallelState = reducer(initialState, {
  type: "event",
  e: { kind: "tool_dispatch", tool: { id: "parallel-1", name: "parallel_tasks", args: "{}", readOnly: false } },
});
parallelState = reducer(parallelState, {
  type: "event",
  e: {
    kind: "tool_dispatch",
    tool: { id: "parallel-1/sub-1", name: "task", args: "{\"description\":\"branch one\"}", readOnly: false, parentId: "parallel-1" },
  },
});
eq(parallelState.items.length, 1, "parallel synthetic branch does not enter parent items");
eq(parallelState.parallelItemsByCall["parallel-1"]?.length, 1, "parallel branch state is owned by the outer call");

const history = historyMessagesToItems([{
  role: "assistant",
  content: "",
  toolCalls: [{
    id: "task-history", name: "task", arguments: "{}", subagentRef: "child-history",
    subagentMessages: [{ role: "assistant", content: "hidden child history" }],
  }],
}, {
  role: "tool", toolCallId: "task-history", content: "child result",
}], "h");
eq(history.items.length, 1, "history reload projects only the outer task card");
ok(history.items.every((item) => item.kind !== "assistant"), "history reload does not append child assistant messages");

let lifecycleState = reducer(initialState, {
  type: "event",
  e: { kind: "tool_dispatch", tool: { id: "task-lifecycle-1", name: "task", args: "{}", readOnly: false } },
});
lifecycleState = reducer(lifecycleState, {
  type: "event",
  e: {
    kind: "tool_result",
    tool: {
      id: "task-lifecycle-1", name: "task", readOnly: false,
      output: "Subagent reference: child-lifecycle\nTask run ID: task-11\nDependencies: task-9\nTurn status: running",
    },
  },
});
lifecycleState = reducer(lifecycleState, {
  type: "event",
  e: { kind: "tool_dispatch", tool: { id: "wait-lifecycle-1", name: "wait_task", args: "{}", readOnly: false } },
});
lifecycleState = reducer(lifecycleState, {
  type: "event",
  e: {
    kind: "tool_result",
    tool: {
      id: "wait-lifecycle-1", name: "wait_task", readOnly: false,
      output: JSON.stringify({ subagentRef: "child-lifecycle", taskRunId: "task-11", dependsOn: ["task-9"], turnStatus: "completed", finalAnswer: "child delivery" }),
    },
  },
});
const lifecycleTask = lifecycleState.items.find((item) => item.kind === "tool" && item.id === "task-lifecycle-1");
const lifecycleWait = lifecycleState.items.find((item) => item.kind === "tool" && item.id === "wait-lifecycle-1");
eq(lifecycleTask?.kind === "tool" ? lifecycleTask.subagentRef : undefined, "child-lifecycle", "task start binds the stable child ref");
eq(lifecycleTask?.kind === "tool" ? lifecycleTask.taskLifecycle?.turnStatus : undefined, "completed", "wait receipt settles the owning task card");
eq(lifecycleTask?.kind === "tool" ? lifecycleTask.taskLifecycle?.finalAnswer : undefined, "child delivery", "task card owns the child delivery");
eq(lifecycleTask?.kind === "tool" ? lifecycleTask.taskLifecycle?.taskRunId : undefined, "task-11", "task card preserves the current run identity");
eq(lifecycleTask?.kind === "tool" ? lifecycleTask.taskLifecycle?.dependsOn?.[0] : undefined, "task-9", "task card preserves accepted dependencies");
eq(lifecycleWait?.kind === "tool" ? lifecycleWait.taskLifecycleRefs?.[0] : undefined, "child-lifecycle", "wait control is marked as folded into the task lifecycle");

let schedulingState = reducer(initialState, {
  type: "event",
  e: { kind: "tool_dispatch", tool: { id: "task-scheduling", name: "task", args: "{}", readOnly: false } },
});
schedulingState = reducer(schedulingState, {
  type: "event",
  e: { kind: "tool_result", tool: { id: "task-scheduling", name: "task", readOnly: false, output: JSON.stringify({ status: "dispatch_decision_required", running_task_runs: ["task-11"] }) } },
});
const schedulingTask = schedulingState.items.find((item) => item.kind === "tool" && item.id === "task-scheduling");
eq(schedulingTask?.kind === "tool" ? schedulingTask.subagentRef : undefined, undefined, "non-dispatch scheduling result creates no child navigation identity");
eq(Object.keys(schedulingState.childItemsByRef).length, 0, "non-dispatch scheduling result creates no child projection");

let pagedLifecycleState = reducer(initialState, {
  type: "history",
  messages: [{
    role: "assistant",
    content: "",
    toolCalls: [{
      id: "wait-page", name: "wait_task", arguments: "{}",
      taskLifecycleReceipts: [{ subagentRef: "child-page", turnStatus: "completed", finalAnswer: "paged delivery" }],
    }],
  }, { role: "tool", toolCallId: "wait-page", content: "receipt" }],
});
let pagedWait = pagedLifecycleState.items.find((item) => item.kind === "tool" && item.id === "wait-page");
eq(pagedWait?.kind === "tool" ? pagedWait.taskLifecycleRefs : undefined, undefined, "unmatched paged wait remains visible");
pagedLifecycleState = reducer(pagedLifecycleState, {
  type: "history_prepend",
  messages: [{
    role: "assistant",
    content: "",
    toolCalls: [{
      id: "task-page", name: "task", arguments: "{}", subagentRef: "child-page",
      taskLifecycle: { subagentRef: "child-page", turnStatus: "running" },
    }],
  }, { role: "tool", toolCallId: "task-page", content: "started" }],
  before: 1,
  hasMore: false,
  generation: "generation-1",
});
const pagedTask = pagedLifecycleState.items.find((item) => item.kind === "tool" && item.id === "task-page");
pagedWait = pagedLifecycleState.items.find((item) => item.kind === "tool" && item.id === "wait-page");
eq(pagedTask?.kind === "tool" ? pagedTask.taskLifecycle?.finalAnswer : undefined, "paged delivery", "prepending the owning task folds a later-page receipt");
eq(pagedWait?.kind === "tool" ? pagedWait.taskLifecycleRefs?.[0] : undefined, "child-page", "cross-page wait hides only after its task is present");

const verifiedArtifactReceipt: TaskLifecycle = {
  subagentRef: "child-artifact",
  turnStatus: "completed",
  finalAnswer: "child prose must not be parsed: C:\\workspace\\ignored.html",
  artifacts: [
    { path: "C:\\workspace\\deliverable.html", size: 42 },
    { path: "c:/workspace/deliverable.html", size: 42 },
    { path: "C:\\workspace\\internal.log", size: 7 },
  ],
  validations: [
    { kind: "turn_status", success: true, status: "completed" },
    { kind: "artifact_observed", success: true, path: "c:/workspace/deliverable.html" },
    { kind: "artifact_observed", success: false, path: "C:\\workspace\\internal.log" },
  ],
};

const userDeliverableReceipt: TaskLifecycle = {
  subagentRef: "writer-artifact",
  turnStatus: "completed",
  artifacts: [
    { path: "C:\\workspace\\artifact-final.html", size: 42, source: "writer" },
    { path: "C:\\workspace\\reports\\browser-run.log", size: 7, source: "snapshot" },
    { path: "C:\\workspace\\reports\\results.yml", size: 9, source: "snapshot" },
    { path: "C:\\workspace\\.github\\workflows\\release.yml", size: 11, source: "writer" },
  ],
  validations: [
    { kind: "turn_status", success: true, status: "completed" },
    { kind: "artifact_observed", success: true, path: "C:\\workspace\\artifact-final.html" },
    { kind: "artifact_observed", success: true, path: "C:\\workspace\\reports\\browser-run.log" },
    { kind: "artifact_observed", success: true, path: "C:\\workspace\\reports\\results.yml" },
    { kind: "artifact_observed", success: true, path: "C:\\workspace\\.github\\workflows\\release.yml" },
  ],
};

const hiddenToolSupportReceipt: TaskLifecycle = {
  subagentRef: "tester-artifact",
  turnStatus: "completed",
  artifacts: [
    { path: "C:\\workspace\\.playwright-cli\\console-private.log", size: 7, source: "snapshot" },
    { path: "C:\\workspace\\.playwright-cli\\page-private.yml", size: 9, source: "snapshot" },
  ],
  validations: [
    { kind: "turn_status", success: true, status: "completed" },
    { kind: "artifact_observed", success: true, path: "C:\\workspace\\.playwright-cli\\console-private.log" },
    { kind: "artifact_observed", success: true, path: "C:\\workspace\\.playwright-cli\\page-private.yml" },
  ],
};

const legacyHiddenSupportReceipt: TaskLifecycle = {
  subagentRef: "legacy-tester-artifact",
  turnStatus: "completed",
  artifacts: [
    { path: "C:\\workspace\\.playwright-cli\\legacy-private.yml", size: 5 },
  ],
  validations: [
    { kind: "turn_status", success: true, status: "completed" },
    { kind: "artifact_observed", success: true, path: "C:\\workspace\\.playwright-cli\\legacy-private.yml" },
  ],
};

const visibleMixedArtifactPaths = [
  "C:\\workspace\\artifact-final.html",
  "C:\\workspace\\reports\\browser-run.log",
  "C:\\workspace\\reports\\results.yml",
  "C:\\workspace\\.github\\workflows\\release.yml",
];

const deliveredFilePaths = [
  "C:\\workspace\\exports\\report.html",
  "C:\\workspace\\exports\\summary.pdf",
];

function assistantArtifactPaths(items: typeof initialState.items, text: string): string[] | undefined {
  const assistant = items.find((item) => item.kind === "assistant" && item.text === text);
  return assistant?.kind === "assistant" ? (assistant as typeof assistant & { artifactPaths?: string[] }).artifactPaths : undefined;
}

function assistantArtifactDelivery(items: typeof initialState.items, text: string) {
  const assistant = items.find((item) => item.kind === "assistant" && item.text === text);
  return assistant?.kind === "assistant" ? assistant.artifactDelivery : undefined;
}

function assistantIsFinalAnswer(items: typeof initialState.items, text: string): boolean {
  const assistant = items.find((item) => item.kind === "assistant" && item.text === text);
  return assistant?.kind === "assistant" && (assistant as typeof assistant & { finalAnswer?: boolean }).finalAnswer === true;
}

function liveItemsForReceipt(receipt: Record<string, unknown>, options: { assistantBefore?: boolean; assistantAfter?: string; nextUser?: boolean } = {}) {
  let state = reducer(initialState, { type: "user", text: "create a deliverable", seq: 0 });
  if (options.assistantBefore) state = reducer(state, { type: "event", e: { kind: "message", text: "before receipt" } });
  state = reducer(state, { type: "event", e: { kind: "tool_dispatch", tool: { id: "wait-artifact", name: "wait_task", args: "{}", readOnly: true } } });
  state = reducer(state, { type: "event", e: { kind: "tool_result", tool: { id: "wait-artifact", name: "wait_task", readOnly: true, output: JSON.stringify(receipt) } } });
  if (options.nextUser) state = reducer(state, { type: "user", text: "next turn", seq: state.seq });
  if (options.assistantAfter) state = reducer(state, { type: "event", e: { kind: "message", text: options.assistantAfter } });
  return state.items;
}

let liveArtifactState = reducer(initialState, { type: "user", text: "create a deliverable", seq: 0 });
liveArtifactState = reducer(liveArtifactState, { type: "event", e: { kind: "turn_started" } });
liveArtifactState = reducer(liveArtifactState, { type: "event", e: { kind: "tool_dispatch", tool: { id: "wait-artifact", name: "wait_task", args: "{}", readOnly: true } } });
liveArtifactState = reducer(liveArtifactState, { type: "event", e: { kind: "tool_result", tool: { id: "wait-artifact", name: "wait_task", readOnly: true, output: JSON.stringify(verifiedArtifactReceipt) } } });
liveArtifactState = reducer(liveArtifactState, { type: "event", e: { kind: "message", text: "first answer after receipt" } });
eq(assistantArtifactPaths(liveArtifactState.items, "first answer after receipt"), undefined, "live projection withholds verified artifacts before turn_done");
liveArtifactState = reducer(liveArtifactState, { type: "event", e: { kind: "tool_dispatch", tool: { id: "status-check", name: "read_file", args: "{}", readOnly: true } } });
liveArtifactState = reducer(liveArtifactState, { type: "event", e: { kind: "tool_result", tool: { id: "status-check", name: "read_file", readOnly: true, output: "ok" } } });
liveArtifactState = reducer(liveArtifactState, { type: "event", e: { kind: "message", text: "final answer after receipt" } });
eq(assistantArtifactPaths(liveArtifactState.items, "first answer after receipt"), undefined, "live projection leaves an earlier post-receipt assistant unchanged");
eq(assistantArtifactPaths(liveArtifactState.items, "final answer after receipt"), undefined, "live projection withholds the last candidate before turn_done");
liveArtifactState = reducer(liveArtifactState, { type: "event", e: { kind: "turn_done" } });
eq(JSON.stringify(assistantArtifactPaths(liveArtifactState.items, "final answer after receipt")), JSON.stringify(["C:\\workspace\\deliverable.html"]), "successful turn_done publishes only the last root candidate");
ok(assistantIsFinalAnswer(liveArtifactState.items, "final answer after receipt"), "successful turn_done marks the accepted root candidate final");

let liveDeliveryState = reducer(initialState, { type: "user", text: "deliver root files", seq: 0 });
liveDeliveryState = reducer(liveDeliveryState, { type: "event", e: { kind: "turn_started", turnId: "delivery-turn" } });
liveDeliveryState = reducer(liveDeliveryState, { type: "event", e: { kind: "message", turnId: "delivery-turn", candidateId: "candidate-before-delivery", text: "candidate before delivery" } });
liveDeliveryState = reducer(liveDeliveryState, { type: "event", e: { kind: "tool_dispatch", tool: { id: "deliver-root", name: "deliver_files", args: "{}", readOnly: true } } });
liveDeliveryState = reducer(liveDeliveryState, { type: "event", e: { kind: "tool_result", tool: { id: "deliver-root", name: "deliver_files", readOnly: true, artifactPaths: [...deliveredFilePaths, "c:/workspace/exports/report.html"] } } });
const liveDeliveryTool = liveDeliveryState.items.find((item) => item.kind === "tool" && item.id === "deliver-root");
eq(liveDeliveryTool?.kind === "tool" ? JSON.stringify(liveDeliveryTool.artifactPaths) : undefined, JSON.stringify([...deliveredFilePaths, "c:/workspace/exports/report.html"]), "live delivery metadata survives completed tool data compaction");
liveDeliveryState = reducer(liveDeliveryState, { type: "event", e: { kind: "message", turnId: "delivery-turn", candidateId: "first-after-delivery", text: "first answer after delivery" } });
eq(assistantArtifactPaths(liveDeliveryState.items, "first answer after delivery"), undefined, "live delivery remains withheld before turn_done");
liveDeliveryState = reducer(liveDeliveryState, { type: "event", e: { kind: "tool_dispatch", tool: { id: "delivery-status", name: "read_file", args: "{}", readOnly: true } } });
liveDeliveryState = reducer(liveDeliveryState, { type: "event", e: { kind: "tool_result", tool: { id: "delivery-status", name: "read_file", readOnly: true, output: "ok" } } });
liveDeliveryState = reducer(liveDeliveryState, { type: "event", e: { kind: "message", turnId: "delivery-turn", candidateId: "final-after-delivery", text: "final answer after delivery" } });
liveDeliveryState = reducer(liveDeliveryState, { type: "event", e: { kind: "turn_done", turnId: "delivery-turn" } });
eq(assistantArtifactPaths(liveDeliveryState.items, "candidate before delivery"), undefined, "delivery does not attach to an assistant before the tool result");
eq(assistantArtifactPaths(liveDeliveryState.items, "first answer after delivery"), undefined, "delivery leaves an earlier post-delivery candidate unchanged");
eq(JSON.stringify(assistantArtifactPaths(liveDeliveryState.items, "final answer after delivery")), JSON.stringify(deliveredFilePaths), "successful turn_done publishes deduplicated root delivery paths on the accepted final assistant");

let otherToolDeliveryState = reducer(initialState, { type: "user", text: "ignore metadata from another tool", seq: 0 });
otherToolDeliveryState = reducer(otherToolDeliveryState, { type: "event", e: { kind: "turn_started" } });
otherToolDeliveryState = reducer(otherToolDeliveryState, { type: "event", e: { kind: "tool_dispatch", tool: { id: "not-delivery", name: "read_file", args: "{}", readOnly: true } } });
otherToolDeliveryState = reducer(otherToolDeliveryState, { type: "event", e: { kind: "tool_result", tool: { id: "not-delivery", name: "read_file", readOnly: true, artifactPaths: deliveredFilePaths } } });
otherToolDeliveryState = reducer(otherToolDeliveryState, { type: "event", e: { kind: "message", text: "other tool answer" } });
otherToolDeliveryState = reducer(otherToolDeliveryState, { type: "event", e: { kind: "turn_done" } });
eq(assistantArtifactPaths(otherToolDeliveryState.items, "other tool answer"), undefined, "artifactPaths from tools other than deliver_files are ignored");

let childDeliveryState = reducer(initialState, { type: "user", text: "ignore a child delivery", seq: 0 });
childDeliveryState = reducer(childDeliveryState, { type: "event", e: { kind: "turn_started" } });
childDeliveryState = reducer(childDeliveryState, { type: "event", e: { kind: "tool_dispatch", tool: { id: "child-delivery", sourceToolId: "child-delivery", parentId: "parent-task", subagentRef: "child-ref", name: "deliver_files", args: "{}", readOnly: true } } });
childDeliveryState = reducer(childDeliveryState, { type: "event", e: { kind: "tool_result", tool: { id: "child-delivery", sourceToolId: "child-delivery", parentId: "parent-task", subagentRef: "child-ref", name: "deliver_files", readOnly: true, artifactPaths: deliveredFilePaths } } });
childDeliveryState = reducer(childDeliveryState, { type: "event", e: { kind: "message", text: "root answer after child delivery" } });
childDeliveryState = reducer(childDeliveryState, { type: "event", e: { kind: "turn_done" } });
eq(assistantArtifactPaths(childDeliveryState.items, "root answer after child delivery"), undefined, "child deliver_files metadata never projects onto the root final assistant");

let failedDeliveryState = reducer(initialState, { type: "user", text: "fail delivery tool", seq: 0 });
failedDeliveryState = reducer(failedDeliveryState, { type: "event", e: { kind: "turn_started" } });
failedDeliveryState = reducer(failedDeliveryState, { type: "event", e: { kind: "tool_dispatch", tool: { id: "failed-delivery", name: "deliver_files", args: "{}", readOnly: true } } });
failedDeliveryState = reducer(failedDeliveryState, { type: "event", e: { kind: "tool_result", tool: { id: "failed-delivery", name: "deliver_files", readOnly: true, err: "verification failed", artifactPaths: deliveredFilePaths } } });
failedDeliveryState = reducer(failedDeliveryState, { type: "event", e: { kind: "message", text: "answer after failed delivery" } });
failedDeliveryState = reducer(failedDeliveryState, { type: "event", e: { kind: "turn_done" } });
eq(assistantArtifactPaths(failedDeliveryState.items, "answer after failed delivery"), undefined, "failed deliver_files results cannot publish artifact paths on an otherwise successful turn");

let committedDeliveryState = reducer(initialState, { type: "user", text: "create a Markdown deliverable", seq: 0 });
committedDeliveryState = reducer(committedDeliveryState, { type: "event", e: { kind: "turn_started" } });
committedDeliveryState = reducer(committedDeliveryState, { type: "event", e: { kind: "tool_dispatch", tool: { id: "wait-committed-delivery", name: "wait_task", args: "{}", readOnly: true } } });
committedDeliveryState = reducer(committedDeliveryState, { type: "event", e: { kind: "tool_result", tool: { id: "wait-committed-delivery", name: "wait_task", readOnly: true, output: JSON.stringify(verifiedArtifactReceipt) } } });
committedDeliveryState = reducer(committedDeliveryState, { type: "event", e: { kind: "message", text: "committed Markdown delivery" } });
committedDeliveryState = reducer(committedDeliveryState, { type: "event", e: { kind: "turn_done", artifactDelivery: { version: 1, paths: ["joke.md"], deliveredAt: 1234 } } });
eq(JSON.stringify(assistantArtifactDelivery(committedDeliveryState.items, "committed Markdown delivery")), JSON.stringify({ version: 1, paths: ["joke.md"], deliveredAt: 1234 }), "successful turn_done binds the complete host artifact delivery to the accepted final answer");
eq(assistantArtifactPaths(committedDeliveryState.items, "committed Markdown delivery"), undefined, "committed delivery does not merge legacy task receipt paths");

let hiddenSupportLiveState = reducer(initialState, { type: "user", text: "create and test a deliverable", seq: 0 });
hiddenSupportLiveState = reducer(hiddenSupportLiveState, { type: "event", e: { kind: "turn_started" } });
hiddenSupportLiveState = reducer(hiddenSupportLiveState, { type: "event", e: { kind: "tool_dispatch", tool: { id: "wait-writer-artifact", name: "wait_task", args: "{}", readOnly: true } } });
hiddenSupportLiveState = reducer(hiddenSupportLiveState, { type: "event", e: { kind: "tool_result", tool: { id: "wait-writer-artifact", name: "wait_task", readOnly: true, output: JSON.stringify(userDeliverableReceipt) } } });
hiddenSupportLiveState = reducer(hiddenSupportLiveState, { type: "event", e: { kind: "tool_dispatch", tool: { id: "wait-tester-artifact", name: "wait_task", args: "{}", readOnly: true } } });
hiddenSupportLiveState = reducer(hiddenSupportLiveState, { type: "event", e: { kind: "tool_result", tool: { id: "wait-tester-artifact", name: "wait_task", readOnly: true, output: JSON.stringify(hiddenToolSupportReceipt) } } });
hiddenSupportLiveState = reducer(hiddenSupportLiveState, { type: "event", e: { kind: "tool_dispatch", tool: { id: "wait-legacy-artifact", name: "wait_task", args: "{}", readOnly: true } } });
hiddenSupportLiveState = reducer(hiddenSupportLiveState, { type: "event", e: { kind: "tool_result", tool: { id: "wait-legacy-artifact", name: "wait_task", readOnly: true, output: JSON.stringify(legacyHiddenSupportReceipt) } } });
hiddenSupportLiveState = reducer(hiddenSupportLiveState, { type: "event", e: { kind: "message", text: "delivery and testing complete" } });
eq(assistantArtifactPaths(hiddenSupportLiveState.items, "delivery and testing complete"), undefined, "mixed artifacts remain withheld before turn_done");
hiddenSupportLiveState = reducer(hiddenSupportLiveState, { type: "event", e: { kind: "turn_done" } });
eq(JSON.stringify(assistantArtifactPaths(hiddenSupportLiveState.items, "delivery and testing complete")), JSON.stringify(visibleMixedArtifactPaths), "live projection keeps explicit hidden writer artifacts and ordinary files while excluding snapshot and legacy hidden support");

let syntheticPlanTodoState = reducer(initialState, { type: "user", text: "complete the approved plan", seq: 0 });
syntheticPlanTodoState = reducer(syntheticPlanTodoState, { type: "event", e: { kind: "turn_started" } });
syntheticPlanTodoState = reducer(syntheticPlanTodoState, { type: "event", e: { kind: "tool_dispatch", tool: { id: "wait-plan-artifact", name: "wait_task", args: "{}", readOnly: true } } });
syntheticPlanTodoState = reducer(syntheticPlanTodoState, { type: "event", e: { kind: "tool_result", tool: { id: "wait-plan-artifact", name: "wait_task", readOnly: true, output: JSON.stringify(verifiedArtifactReceipt) } } });
syntheticPlanTodoState = reducer(syntheticPlanTodoState, { type: "event", e: { kind: "message", text: "accepted answer before host todo completion" } });
eq(assistantArtifactPaths(syntheticPlanTodoState.items, "accepted answer before host todo completion"), undefined, "host todo completion path withholds artifacts before turn_done");
syntheticPlanTodoState = reducer(syntheticPlanTodoState, { type: "event", e: { kind: "tool_dispatch", tool: { id: "plan-seed", name: "todo_write", args: JSON.stringify({ todos: [{ content: "finish", status: "completed" }] }), readOnly: true } } });
syntheticPlanTodoState = reducer(syntheticPlanTodoState, { type: "event", e: { kind: "tool_result", tool: { id: "plan-seed", name: "todo_write", readOnly: true, output: "approved plan finished" } } });
eq(assistantArtifactPaths(syntheticPlanTodoState.items, "accepted answer before host todo completion"), undefined, "host todo completion still withholds artifacts before turn_done");
syntheticPlanTodoState = reducer(syntheticPlanTodoState, { type: "event", e: { kind: "turn_done" } });
ok(assistantIsFinalAnswer(syntheticPlanTodoState.items, "accepted answer before host todo completion"), "host synthetic plan-seed preserves the accepted root candidate");
eq(JSON.stringify(assistantArtifactPaths(syntheticPlanTodoState.items, "accepted answer before host todo completion")), JSON.stringify(["C:\\workspace\\deliverable.html"]), "successful turn_done publishes artifacts after host synthetic plan-seed");

let failedModelToolState = reducer(initialState, { type: "user", text: "fail after a model tool call", seq: 0 });
failedModelToolState = reducer(failedModelToolState, { type: "event", e: { kind: "turn_started" } });
failedModelToolState = reducer(failedModelToolState, { type: "event", e: { kind: "tool_dispatch", tool: { id: "wait-failed-tool-artifact", name: "wait_task", args: "{}", readOnly: true } } });
failedModelToolState = reducer(failedModelToolState, { type: "event", e: { kind: "tool_result", tool: { id: "wait-failed-tool-artifact", name: "wait_task", readOnly: true, output: JSON.stringify(verifiedArtifactReceipt) } } });
failedModelToolState = reducer(failedModelToolState, { type: "event", e: { kind: "message", text: "preface before model tool" } });
failedModelToolState = reducer(failedModelToolState, { type: "event", e: { kind: "tool_dispatch", tool: { id: "model-write", name: "write_file", args: "{}", readOnly: false } } });
failedModelToolState = reducer(failedModelToolState, { type: "event", e: { kind: "tool_result", tool: { id: "model-write", name: "write_file", readOnly: false, output: "write failed", err: "disk full" } } });
failedModelToolState = reducer(failedModelToolState, { type: "event", e: { kind: "turn_done", err: "model tool failed" } });
eq(assistantArtifactPaths(failedModelToolState.items, "preface before model tool"), undefined, "failed turn after a model tool call does not publish artifacts");
ok(!assistantIsFinalAnswer(failedModelToolState.items, "preface before model tool"), "failed turn after a model tool call does not mark its message final");

const terminalScenarios: Array<{
  label: string;
  done: WireEvent;
  planApproval?: boolean;
  omitAssistant?: boolean;
  reject?: boolean;
}> = [
  { label: "failed", done: { kind: "turn_done", err: "provider failed", artifactDelivery: { version: 1, paths: ["failed.md"] } } },
  { label: "interrupted", done: { kind: "turn_done", interrupted: true, artifactDelivery: { version: 1, paths: ["interrupted.md"] } } },
  { label: "plan approval", done: { kind: "turn_done", artifactDelivery: { version: 1, paths: ["plan.md"] } }, planApproval: true },
  { label: "no assistant", done: { kind: "turn_done", artifactDelivery: { version: 1, paths: ["missing.md"] } }, omitAssistant: true },
  { label: "rejected candidate", done: { kind: "turn_done", artifactDelivery: { version: 1, paths: ["rejected.md"] } }, reject: true },
];
for (const terminal of terminalScenarios) {
  let state = reducer(initialState, { type: "user", text: `create a ${terminal.label} deliverable`, seq: 0 });
  state = reducer(state, { type: "event", e: { kind: "turn_started" } });
  state = reducer(state, { type: "event", e: { kind: "tool_dispatch", tool: { id: `deliver-${terminal.label}`, name: "deliver_files", args: "{}", readOnly: true } } });
  state = reducer(state, { type: "event", e: { kind: "tool_result", tool: { id: `deliver-${terminal.label}`, name: "deliver_files", readOnly: true, artifactPaths: deliveredFilePaths } } });
  if (!terminal.omitAssistant) state = reducer(state, { type: "event", e: { kind: "message", text: `${terminal.label} terminal candidate` } });
  if (terminal.reject) state = reducer(state, { type: "event", e: { kind: "final_candidate_rejected" } });
  if (terminal.planApproval) state = reducer(state, { type: "event", e: { kind: "approval_request", approval: { id: "plan-artifact", tool: "exit_plan_mode", subject: "Approve plan" } } });
  state = reducer(state, { type: "event", e: terminal.done });
  ok(state.items.every((item) => item.kind !== "assistant" || !item.artifactPaths?.length), `${terminal.label} turn_done does not publish structured artifacts`);
  ok(state.items.every((item) => item.kind !== "assistant" || item.artifactDelivery === undefined), `${terminal.label} turn_done does not bind a delivery contract`);
  ok(state.items.every((item) => item.kind !== "assistant" || item.finalAnswer !== true), `${terminal.label} turn_done does not mark a candidate final`);
}

const historyArtifactState = reducer(initialState, {
  type: "history",
  messages: [
    { role: "user", content: "create a deliverable" },
    { role: "assistant", content: "history before receipt" },
    { role: "assistant", content: "", toolCalls: [{ id: "wait-history-artifact", name: "wait_task", arguments: "{}", taskLifecycleReceipts: [verifiedArtifactReceipt] }] },
    { role: "tool", toolCallId: "wait-history-artifact", content: "receipt" },
    { role: "assistant", content: "history final answer", finalAnswer: true },
  ],
});
eq(assistantArtifactPaths(historyArtifactState.items, "history before receipt"), undefined, "history projection leaves the pre-receipt assistant unchanged");
eq(JSON.stringify(assistantArtifactPaths(historyArtifactState.items, "history final answer")), JSON.stringify(["C:\\workspace\\deliverable.html"]), "history projection restores an explicitly marked final answer");

const historyDeliveryState = reducer(initialState, {
  type: "history",
  messages: [
    { role: "user", content: "restore a root delivery" },
    { role: "assistant", content: "history before delivery" },
    { role: "assistant", content: "", toolCalls: [{ id: "deliver-history", name: "deliver_files", arguments: "", artifactPaths: deliveredFilePaths }] },
    { role: "tool", toolCallId: "deliver-history", toolName: "deliver_files", content: "", toolResultArchived: true },
    { role: "assistant", content: "history delivery final answer", finalAnswer: true },
  ],
});
const historyDeliveryTool = historyDeliveryState.items.find((item) => item.kind === "tool" && item.id === "deliver-history");
ok(historyDeliveryTool?.kind === "tool" && historyDeliveryTool.readOnly, "history restores deliver_files with its read-only execution contract");
eq(assistantArtifactPaths(historyDeliveryState.items, "history before delivery"), undefined, "history delivery leaves an assistant before deliver_files unchanged");
eq(JSON.stringify(assistantArtifactPaths(historyDeliveryState.items, "history delivery final answer")), JSON.stringify(deliveredFilePaths), "history restores explicit delivery metadata even when the successful tool output is archived");

const invalidHistoryDeliveryState = reducer(initialState, {
  type: "history",
  messages: [
    { role: "user", content: "ignore invalid historical delivery metadata" },
    { role: "assistant", content: "", toolCalls: [{ id: "other-history", name: "read_file", arguments: "", artifactPaths: deliveredFilePaths }] },
    { role: "tool", toolCallId: "other-history", toolName: "read_file", content: "", toolResultArchived: true },
    { role: "assistant", content: "", toolCalls: [{ id: "failed-history-delivery", name: "deliver_files", arguments: "", artifactPaths: deliveredFilePaths }] },
    { role: "tool", toolCallId: "failed-history-delivery", toolName: "deliver_files", content: "verification failed", toolResultError: "verification failed" },
    { role: "assistant", content: "invalid history delivery answer", finalAnswer: true },
  ],
});
eq(assistantArtifactPaths(invalidHistoryDeliveryState.items, "invalid history delivery answer"), undefined, "history ignores artifactPaths from other tools and failed deliver_files results");

const historyCommittedDeliveryState = reducer(initialState, {
  type: "history",
  messages: [
    { role: "user", content: "create a Markdown deliverable" },
    { role: "assistant", content: "", toolCalls: [{ id: "wait-history-committed", name: "wait_task", arguments: "{}", taskLifecycleReceipts: [verifiedArtifactReceipt] }] },
    { role: "tool", toolCallId: "wait-history-committed", content: "receipt" },
    { role: "assistant", content: "history committed Markdown delivery", finalAnswer: true, artifactDelivery: { version: 1, paths: ["joke.md"], deliveredAt: 5678 } },
  ],
});
eq(JSON.stringify(assistantArtifactDelivery(historyCommittedDeliveryState.items, "history committed Markdown delivery")), JSON.stringify({ version: 1, paths: ["joke.md"], deliveredAt: 5678 }), "history projection preserves the complete committed artifact delivery");
eq(assistantArtifactPaths(historyCommittedDeliveryState.items, "history committed Markdown delivery"), undefined, "history committed delivery suppresses legacy receipt projection");

const hiddenSupportHistoryState = reducer(initialState, {
  type: "history",
  messages: [
    { role: "user", content: "create and test a historical deliverable" },
    { role: "assistant", content: "", toolCalls: [{ id: "wait-history-writer", name: "wait_task", arguments: "{}", taskLifecycleReceipts: [userDeliverableReceipt] }] },
    { role: "tool", toolCallId: "wait-history-writer", content: "writer receipt" },
    { role: "assistant", content: "", toolCalls: [{ id: "wait-history-tester", name: "wait_task", arguments: "{}", taskLifecycleReceipts: [hiddenToolSupportReceipt] }] },
    { role: "tool", toolCallId: "wait-history-tester", content: "tester receipt" },
    { role: "assistant", content: "", toolCalls: [{ id: "wait-history-legacy", name: "wait_task", arguments: "{}", taskLifecycleReceipts: [legacyHiddenSupportReceipt] }] },
    { role: "tool", toolCallId: "wait-history-legacy", content: "legacy receipt" },
    { role: "assistant", content: "historical delivery complete", finalAnswer: true },
  ],
});
eq(JSON.stringify(assistantArtifactPaths(hiddenSupportHistoryState.items, "historical delivery complete")), JSON.stringify(visibleMixedArtifactPaths), "history projection keeps explicit hidden writer artifacts while legacy hidden receipts fail closed");

const unmarkedHistoryArtifactState = reducer(initialState, {
  type: "history",
  messages: [
    { role: "user", content: "create a legacy deliverable" },
    { role: "assistant", content: "", toolCalls: [{ id: "wait-unmarked-artifact", name: "wait_task", arguments: "{}", taskLifecycleReceipts: [verifiedArtifactReceipt] }] },
    { role: "tool", toolCallId: "wait-unmarked-artifact", content: "receipt" },
    { role: "assistant", content: "unmarked legacy candidate" },
  ],
});
eq(assistantArtifactPaths(unmarkedHistoryArtifactState.items, "unmarked legacy candidate"), undefined, "history projection does not guess an unmarked assistant is final");

for (const [label, receipt] of [
  ["missing validation", { ...verifiedArtifactReceipt, validations: [] }],
  ["failed validation", { ...verifiedArtifactReceipt, validations: [{ kind: "artifact_observed", success: false, path: "C:\\workspace\\deliverable.html" }] }],
  ["mismatched validation", { ...verifiedArtifactReceipt, validations: [{ kind: "artifact_observed", success: true, path: "C:\\workspace\\other.html" }] }],
  ["failed receipt", { ...verifiedArtifactReceipt, turnStatus: "failed" }],
  ["interrupted receipt", { ...verifiedArtifactReceipt, turnStatus: "interrupted" }],
  ["running receipt", { ...verifiedArtifactReceipt, turnStatus: "running" }],
  ["final answer only", { subagentRef: "child-artifact", turnStatus: "completed", finalAnswer: "C:\\workspace\\deliverable.html" }],
] as const) {
  eq(assistantArtifactPaths(liveItemsForReceipt(receipt, { assistantAfter: label }), label), undefined, `${label} does not project an artifact path`);
}
eq(assistantArtifactPaths(liveItemsForReceipt(verifiedArtifactReceipt, { assistantBefore: true }), "before receipt"), undefined, "receipt without a later assistant does not attach to earlier prose");
ok(liveItemsForReceipt(verifiedArtifactReceipt).every((item) => item.kind !== "assistant" || item.text !== ""), "receipt without a later assistant does not fabricate a message");
eq(assistantArtifactPaths(liveItemsForReceipt(verifiedArtifactReceipt, { nextUser: true, assistantAfter: "next turn answer" }), "next turn answer"), undefined, "artifact projection does not cross a user boundary");
let orphanReceiptState = reducer(initialState, { type: "event", e: { kind: "tool_dispatch", tool: { id: "wait-orphan", name: "wait_task", args: "{}", readOnly: true } } });
orphanReceiptState = reducer(orphanReceiptState, { type: "event", e: { kind: "tool_result", tool: { id: "wait-orphan", name: "wait_task", readOnly: true, output: JSON.stringify(verifiedArtifactReceipt) } } });
orphanReceiptState = reducer(orphanReceiptState, { type: "event", e: { kind: "message", text: "assistant without visible user boundary" } });
eq(assistantArtifactPaths(orphanReceiptState.items, "assistant without visible user boundary"), undefined, "artifact projection fails closed without a user-turn boundary");

const toolCardSource = readFileSync(new URL("../components/ToolCard.tsx", import.meta.url), "utf8");
ok(toolCardSource.includes('data-testid="parallel-task-list"'), "parallel branches render as an aggregate list");
ok(!toolCardSource.includes("<AgentWorkspace"), "parallel card no longer creates AgentWorkspace tabs");
ok(!toolCardSource.includes("SubagentHistoryForTab"), "ordinary task card no longer exposes child history");

if (failed > 0) {
  console.error(`\n${failed} failed, ${passed} passed`);
  process.exit(1);
}
console.log(`\n${passed} passed`);
