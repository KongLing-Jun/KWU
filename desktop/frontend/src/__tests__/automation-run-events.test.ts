// Run: tsx src/__tests__/automation-run-events.test.ts

import assert from "node:assert/strict";
import { automationRunEventLabel, automationRunEventText, normalizeAutomationRunEvents } from "../features/automation/heartbeat/HeartbeatPanel";

assert.equal(automationRunEventLabel({ status: "succeeded" }), "成功");
assert.equal(automationRunEventLabel({ kind: 1 }), "思考");
assert.equal(automationRunEventLabel({ kind: 2 }), "输出");
for (const [kind, label] of [
  [18, "已退役事件"],
  [19, "安全评估"],
  [20, "审计"],
  [21, "任务树"],
  [22, "任务更新"],
  [23, "任务开始"],
  [24, "任务输出"],
  [25, "任务完成"],
  [26, "任务失败"],
  [27, "任务树完成"],
  [28, "任务摘要"],
] as const) {
  assert.equal(automationRunEventLabel({ kind }), label);
}
assert.equal(automationRunEventLabel({ level: "warning" }), "警告");
assert.equal(automationRunEventLabel({}), "事件");

assert.equal(automationRunEventText({ text: "开始执行任务" }), "开始执行任务");
assert.equal(automationRunEventText({ error: "数据源缺失", text: "ignored" }), "数据源缺失");
assert.equal(automationRunEventText({}), "无事件详情");

const normalized = normalizeAutomationRunEvents([
  { time: "2026-06-29T07:34:00Z", status: "queued", text: "automation run queued" },
  { time: "2026-06-29T07:34:01Z", kind: 1, level: 0, text: "This" },
  { time: "2026-06-29T07:34:01Z", kind: 1, level: 0, text: "is" },
  { time: "2026-06-29T07:34:01Z", kind: 1, level: 0, text: "a" },
  { time: "2026-06-29T07:34:01Z", kind: 1, level: 0, text: "每日" },
  { time: "2026-06-29T07:34:01Z", kind: 1, level: 0, text: "一个" },
  { time: "2026-06-29T07:34:01Z", kind: 1, level: 0, text: "为什么" },
  { time: "2026-06-29T07:34:02Z", status: "running", text: "automation run running" },
]);

assert.equal(normalized.length, 3);
assert.equal(normalized[1].text, "This is a 每日一个为什么");

const taskEvents = normalizeAutomationRunEvents([
  { kind: 24, level: 0, text: "task" },
  { kind: 24, level: 0, text: "output" },
  { kind: 23, level: 0, text: "started one" },
  { kind: 23, level: 0, text: "started two" },
  { kind: "task_node_output", level: 0, text: "wire" },
  { kind: "task_node_output", level: 0, text: "output" },
]);
assert.equal(taskEvents.length, 4);
assert.equal(taskEvents[0].text, "task output");
assert.equal(taskEvents[1].text, "started one");
assert.equal(taskEvents[2].text, "started two");
assert.equal(taskEvents[3].text, "wire output");

console.log("automation run events render contract passed");
