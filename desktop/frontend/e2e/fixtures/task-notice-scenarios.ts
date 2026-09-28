import { applyEvent } from "../../src/features/session/eventProjection";
import { foldTaskLifecycleItems } from "../../src/features/session/historyProjection";
import { initialState } from "../../src/features/session/sessionState";
import type { Item, ToolItem } from "../../src/features/session/sessionItems";

const failure = "Review stopped after reaching the step limit.";

export function failureEnvelope(text = failure) {
  return {
    jsonrpc: "2.0", id: "a2a-request-1",
    result: { task: {
      id: "a2a-task-1", contextId: "fixture-context",
      status: {
        state: "TASK_STATE_FAILED", timestamp: "2026-09-09T01:00:00Z",
        message: {
          messageId: "a2a-message-1", taskId: "a2a-task-1", contextId: "fixture-context", role: "ROLE_AGENT",
          parts: [{ text, mediaType: "text/plain" }],
        },
      },
      history: [{ messageId: "a2a-request-1", contextId: "fixture-context", role: "ROLE_USER", parts: [{ text: "Review the game", mediaType: "text/plain" }] }],
      metadata: {
        protocolVersion: "1.0", protocolBinding: "urn:reasonix:a2a:binding:in-process:v1",
        topology: { mode: "team", senderRole: "lead", senderName: "Lead", recipientRole: "member", recipientName: "Reviewer" },
      },
    } },
  };
}

export const scenarios = [
  ["retry", "A2A retry", 0], ["partial", "A2A with partial answer", 0],
  ["plain", "Plain error retry", 0], ["multiline", "Exact multiline error", 0],
  ["same-ref", "Same-reference retry", 1], ["same-ref-plain", "Same-reference plain error", 1],
  ["late-live", "Late receipt: live projection", 1], ["late-history", "Late receipt: history projection", 1],
  ["pending", "Before failure receipt", 1], ["missing", "Missing diagnostics", 1],
  ["unmatched", "Unmatched run", 1], ["extra", "Additional notice diagnostics", 1],
  ["ambiguous", "Ambiguous run owner", 1], ["other", "Unrelated warning", 1],
  ["old", "Archived old turn", 0], ["malformed", "Truncated envelope", 1],
  ["trailing", "Trailing diagnostics", 1], ["markers", "Multiple markers", 1],
  ["embedded", "Embedded marker", 1], ["parts", "Multiple message parts", 1],
  ["data", "Conflicting data part", 1], ["canceled", "Canceled state", 1],
  ["completed", "Completed state", 1], ["interrupted", "Interrupted lifecycle", 1],
  ["version", "Unknown protocol", 1], ["history-only", "Error only in history", 1],
  ["whitespace", "Additional diagnostic whitespace", 1], ["oversize", "Oversize envelope", 1],
  ["missing-ref", "Missing owner reference", 1],
] as const;

export function taskNoticeScenario(scenario: string): Item[] {
  const diagnostic = scenario === "multiline" ? 'First line\nSecond line: "quoted" \\ detail' : failure;
  const envelope = failureEnvelope(diagnostic);
  if (scenario === "parts") envelope.result.task.status.message.parts.push({ text: "More detail", mediaType: "text/plain" });
  if (scenario === "data") Object.assign(envelope.result.task.status.message.parts[0], { data: { detail: true } });
  if (scenario === "canceled") envelope.result.task.status.state = "TASK_STATE_CANCELED";
  if (scenario === "completed") envelope.result.task.status.state = "TASK_STATE_COMPLETED";
  if (scenario === "version") envelope.result.task.metadata.protocolVersion = "2.0";
  if (scenario === "history-only") {
    envelope.result.task.status.message.parts = [];
    envelope.result.task.history[0].parts[0].text = diagnostic;
  }
  let error = `A2A response: ${JSON.stringify(envelope)}`;
  if (scenario === "plain" || scenario === "same-ref-plain") error = diagnostic;
  if (scenario === "partial") error = `Partial review findings remain available.\n\n${error}`;
  if (scenario === "malformed") error = error.slice(0, -1);
  if (scenario === "trailing") error += "\nAdditional failure detail";
  if (scenario === "markers") error = `Quoted A2A response: example\n\n${error}`;
  if (scenario === "embedded") error = `Inline ${error}`;
  if (scenario === "oversize") error = `${"x".repeat(256 * 1024)}\n\n${error}`;
  const task: ToolItem = {
    kind: "tool", id: "review-1", name: "task", args: '{"description":"Review code"}',
    readOnly: false, status: "done", dataArchived: true, subagentRef: "sa_review",
    taskLifecycle: { subagentRef: "sa_review", taskRunId: "job-1", turnStatus: "failed", error },
  };
  if (scenario === "pending") task.taskLifecycle!.turnStatus = "running";
  if (scenario === "interrupted") task.taskLifecycle!.turnStatus = "interrupted";
  if (scenario === "missing") task.taskLifecycle!.error = undefined;
  if (scenario === "missing-ref") task.subagentRef = undefined;
  const notice: Item = {
    kind: "notice", id: "failure-1", level: "warn",
    text: `background task failed: ${scenario === "unmatched" ? "job-10" : "job-1"} \u2014 ${diagnostic}`,
  };
  if (scenario === "extra") notice.text += "\nAdditional failure detail";
  if (scenario === "whitespace") notice.text += " ";
  const sameRef = scenario.startsWith("same-ref") || scenario.startsWith("late-");
  const ref = sameRef ? "sa_review" : "sa_retry";
  const replacement: ToolItem = {
    ...task, id: "review-2", subagentRef: ref,
    taskLifecycle: { subagentRef: ref, taskRunId: "job-2", turnStatus: "running" },
  };
  const items: Item[] = [
    { kind: "user", id: "u1", text: "Review the game" }, task, notice,
    { kind: "assistant", id: "a1", text: "Starting another review with a higher step limit.", reasoning: "", streaming: false }, replacement,
  ];
  if (scenario.startsWith("late-")) {
    const receipt = { ...task.taskLifecycle! };
    task.taskLifecycle = { subagentRef: "sa_review", taskRunId: "job-1", turnStatus: "running" };
    const control: ToolItem = { kind: "tool", id: "wait", name: "wait_task", args: "{}", readOnly: false, status: "running" };
    if (scenario === "late-history") return foldTaskLifecycleItems([...items, { ...control, status: "done", taskLifecycleReceipts: [receipt] }]);
    return applyEvent({ ...initialState, items: [...items, control] }, {
      kind: "tool_result", tool: { id: "wait", name: "wait_task", args: "{}", readOnly: false, output: JSON.stringify(receipt) },
    }).items;
  }
  if (scenario === "ambiguous") items.push({ ...task, id: "duplicate-owner" });
  if (scenario === "other") items.push({ kind: "notice", id: "shell-error", level: "warn", text: "background bash failed: command-1 \u2014 Command failed" });
  if (scenario === "old") {
    for (let index = 0; index < 31; index++) {
      items.push({ kind: "user", id: `old-u${index}`, text: `Later request ${index + 1}` });
      items.push({ kind: "assistant", id: `old-a${index}`, text: "Done.", reasoning: "", streaming: false, finalAnswer: true });
    }
  }
  return items;
}
