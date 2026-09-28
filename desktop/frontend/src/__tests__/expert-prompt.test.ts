// Run: tsx src/__tests__/expert-prompt.test.ts

import { buildExpertSubmitText, stripHiddenSystemPrompt, stripLeadingHiddenSystemPrompt } from "../lib/expertPrompt";
import { compactQuestionText, warmUserPreview } from "../lib/transcriptGrouping";

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

function eq<T>(actual: T, expected: T, label: string) {
  ok(Object.is(actual, expected), `${label}${Object.is(actual, expected) ? "" : ` (got ${JSON.stringify(actual)}, want ${JSON.stringify(expected)})`}`);
}

function hiddenSystemBlock(text: string): string {
  const end = text.indexOf("</system-prompt>");
  return end >= 0 ? text.slice(0, end) : text;
}

console.log("\nexpert hidden system prompt");

const expert = {
  id: "backend-agent-team",
  name: "Backend Agent Team",
  role: "Agent orchestration",
  mode: "team" as const,
  prompt: "Coordinate the backend agent team.",
  systemPrompt: "Use the hidden backend team operating contract.",
  members: [
    { kind: "lead", name: "Lead Agent", role: "Coordinator", task: "Plan and delegate.", output: "Plan", tone: "blue" },
    { kind: "sub", name: "Worker Agent", role: "Execution", task: "Do the assigned work.", output: "Result", tone: "green" },
  ],
};

const displayText = "请把这个需求拆成执行计划";
const submitText = buildExpertSubmitText(displayText, expert);
const hiddenBlock = hiddenSystemBlock(submitText);

eq(displayText, "请把这个需求拆成执行计划", "visible composer text stays as the user question");
ok(submitText.includes("<system-prompt"), "submit text contains a hidden system prompt wrapper");
ok(submitText.includes('source="expert"'), "hidden prompt is marked as expert-sourced");
ok(submitText.includes('member-names="[&quot;Worker Agent&quot;]"'), "hidden prompt carries the configured sub-expert allowlist");
ok(!hiddenBlock.includes("Use the hidden backend team operating contract."), "team hidden prompt does not duplicate the session system prompt");
ok(!hiddenBlock.includes("Coordinate the backend agent team."), "hidden prompt does not duplicate the visible expert prompt");
const backendCatalogSubmitText = buildExpertSubmitText(displayText, { ...expert, systemPrompt: undefined });
const backendCatalogHiddenBlock = hiddenSystemBlock(backendCatalogSubmitText);
ok(backendCatalogHiddenBlock.includes("当前回合已启用专家上下文"), "backend catalog hidden prompt uses a neutral activation marker");
ok(!backendCatalogHiddenBlock.includes("Coordinate the backend agent team."), "backend catalog hidden prompt does not fall back to the visible prompt");
ok(submitText.includes("Worker Agent"), "team routing metadata includes configured subagent names");
for (const term of ["专家团编排规则", "task 子代理工具", "member_name", "complete_step", "deliver_files", "read_only_task", "parallel_tasks", "A2A"]) {
  ok(!hiddenBlock.includes(term), `team hidden prompt does not duplicate runtime rule ${term}`);
}
ok(submitText.includes("待办任务"), "team prompt requires Chinese todo text");
ok(submitText.includes("todos[].content"), "team prompt covers todo content language");
ok(submitText.includes("activeForm"), "team prompt covers active todo form language");
ok(!submitText.includes("职责：Agent orchestration"), "team hidden prompt leaves lead role ownership to the session system prompt");
ok(!submitText.includes("Role:"), "hidden prompt does not inject English role labels");
ok(submitText.endsWith(displayText), "submit text ends with the user question");
ok(!displayText.includes("Coordinate the backend agent team."), "expert prompt is not appended to visible text");
eq(stripHiddenSystemPrompt(submitText), displayText, "hidden expert system prompt is stripped from user-facing text");
const indentedUserText = "    indented code\n  keep nested indentation";
eq(
  stripHiddenSystemPrompt(buildExpertSubmitText(indentedUserText, expert)),
  indentedUserText,
  "stripping the hidden expert prompt preserves user indentation",
);
eq(stripHiddenSystemPrompt(`goal set: ${submitText}`), `goal set: ${displayText}`, "hidden expert system prompt is stripped when embedded in a notice");
eq(stripHiddenSystemPrompt(`目标已设置 → ${submitText.slice(0, 96)}`), "目标已设置 → ", "truncated hidden expert prompt is stripped from notices");
eq(stripLeadingHiddenSystemPrompt(submitText), displayText, "leading hidden expert prompt is projected to visible user text");
eq(
  stripLeadingHiddenSystemPrompt(`quoted control:\n${submitText}`),
  `quoted control:\n${submitText}`,
  "strict display projection preserves non-leading hidden prompt markup",
);
eq(
  stripLeadingHiddenSystemPrompt(`<system-prompt source="expert" hidden="true">\nincomplete`),
  `<system-prompt source="expert" hidden="true">\nincomplete`,
  "strict display projection preserves incomplete leading hidden prompt markup",
);

const soloSubmitText = buildExpertSubmitText(displayText, {
  ...expert,
  mode: "expert",
  systemPrompt: "Use the hidden standalone expert operating contract.",
  prompt: "请按专家模式处理当前任务。",
  members: [],
});
ok(soloSubmitText.includes("待办任务"), "expert mode prompt also requires Chinese todo text");
ok(hiddenSystemBlock(soloSubmitText).includes("Use the hidden standalone expert operating contract."), "expert mode hidden prompt includes the expert system prompt");
ok(soloSubmitText.includes("todos[].content"), "expert mode prompt covers todo content language");
ok(soloSubmitText.includes("activeForm"), "expert mode prompt covers active todo form language");
ok(!soloSubmitText.includes("Role:"), "expert mode prompt does not inject English role labels");

const unnamedMemberSubmitText = buildExpertSubmitText(displayText, {
  ...expert,
  members: [{ kind: "sub", task: "执行分配的任务。", output: "执行结果" }],
});
ok(unnamedMemberSubmitText.includes('member-names="[]"'), "unnamed members are excluded from the executable sub-expert allowlist");
const unnamedSubAfterLead = buildExpertSubmitText(displayText, {
  ...expert,
  members: [
    { kind: "LEAD", name: "Lead Agent" },
    { kind: "sub", task: "执行分配的任务。" },
  ],
});
ok(unnamedSubAfterLead.includes('member-names="[]"'), "malformed kinds and blank names fail closed instead of creating synthetic sub-experts");
const caseCollidingMembers = buildExpertSubmitText(displayText, {
  ...expert,
  members: [
    { kind: "lead", name: "Lead Agent" },
    { kind: "sub", name: "Review Expert" },
    { kind: "sub", name: "review expert" },
  ],
});
ok(caseCollidingMembers.includes('member-names="[]"'), "case-colliding sub-expert names fail closed as an ambiguous configuration");

eq(buildExpertSubmitText(displayText, null), displayText, "missing expert leaves submit text unchanged");
const blankTeamSubmitText = buildExpertSubmitText(displayText, { ...expert, prompt: "   ", systemPrompt: "   " });
ok(blankTeamSubmitText.includes('mode="team"'), "blank team prompts still activate expert-team routing");
ok(blankTeamSubmitText.includes('member-names="[&quot;Worker Agent&quot;]"'), "blank team prompts still carry the sub-expert allowlist");
eq(stripHiddenSystemPrompt(blankTeamSubmitText), displayText, "blank team prompts preserve the visible user text");
eq(buildExpertSubmitText(displayText, { ...expert, mode: "expert", prompt: "   ", systemPrompt: "   " }), displayText, "blank standalone expert prompt leaves submit text unchanged");
eq(compactQuestionText(submitText), displayText, "question navigation hides the expert system prompt");
eq(warmUserPreview(submitText), displayText, "collapsed chat summaries hide the expert system prompt");

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
