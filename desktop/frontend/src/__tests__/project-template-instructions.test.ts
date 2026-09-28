import { projectTemplateInstructions } from "../features/projects/model/projectTemplateInstructions";

let passed = 0;
let failed = 0;

function ok(value: unknown, label: string) {
  if (value) {
    process.stdout.write(`  PASS  ${label}\n`);
    passed += 1;
  } else {
    process.stdout.write(`  FAIL  ${label}\n`);
    failed += 1;
  }
}

console.log("\nproject template instruction contract");

ok(
  JSON.stringify(Object.keys(projectTemplateInstructions)) === JSON.stringify(["report", "research", "meeting", "plan", "review"]),
  "the directory exposes exactly the five supplied templates in product order",
);

const expectations = {
  report: ["## 阶段一：需求规划", "## 阶段六：上线复盘", "遵循 EARS 原则", "# 提醒事项"],
  research: ["## 阶段一：调研课题定义", "## 阶段七：持续跟踪", "区分事实、分析和建议", "# 提醒事项"],
  meeting: ["## 阶段一：知识库初始化", "## 阶段六：知识运营", "当前知识库未提供", "# 提醒事项"],
  plan: ["## 阶段一：售前到交付交接", "## 阶段七：交付复盘", "区分内部信息和客户可见信息", "# 提醒事项"],
  review: ["## 阶段一：测试准备", "## 阶段六：质量复盘", "不在缺少证据时断言根因", "# 提醒事项"],
} as const;

for (const [id, requiredSections] of Object.entries(expectations)) {
  const instruction = projectTemplateInstructions[id as keyof typeof projectTemplateInstructions];
  ok(instruction.startsWith("# 角色\n"), `${id} starts with the supplied role section`);
  ok(requiredSections.every((section) => instruction.includes(section)), `${id} retains every required workflow section`);
  ok(instruction.length > 900, `${id} contains the complete instruction rather than a placeholder`);
}

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
