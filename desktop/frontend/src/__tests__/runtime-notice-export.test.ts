// Run: tsx src/__tests__/runtime-notice-export.test.ts

import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import ts from "typescript";

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

console.log("\nruntime notice export");

const appSource = readFileSync(resolve(import.meta.dirname, "../App.tsx"), "utf8");
const start = appSource.indexOf("function materializeLiveItems(");
const end = appSource.indexOf("function safeFilename(", start);
if (start < 0 || end < 0) throw new Error("session export implementation block not found");

const sourceUnderTest = `
const BRAND_DISPLAY_NAME = "Test";
${appSource.slice(start, end)}
holder.markdown = sessionItemsToMarkdown;
holder.json = sessionItemsToJson;
`;
const compiled = ts.transpileModule(sourceUnderTest, {
  compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2022 },
}).outputText;
const holder: { markdown?: (title: string, items: unknown[]) => string; json?: (title: string, items: unknown[]) => string } = {};
new Function("holder", compiled)(holder);
if (!holder.markdown || !holder.json) throw new Error("session export functions were not loaded");

const rawNotice = "background task failed: task-7 — provider unavailable";
const rawPhase = "provider/model · executing";
const items = [
  { kind: "notice", id: "internal-notice-42", level: "warn", text: rawNotice, forceReveal: true },
  { kind: "phase", id: "p1", text: rawPhase },
  { kind: "tool", id: "t1", name: "parallel_tasks", args: "{\"tasks\":[\"a\",\"b\"]}", output: "temporary aggregate", status: "done" },
];

const markdown = holder.markdown("runtime export", items);
ok(markdown.includes(rawNotice), "Markdown copy preserves raw lifecycle Notice and task ID");
ok(markdown.includes(rawPhase), "Markdown copy preserves raw Phase");
ok(markdown.includes("parallel_tasks") && markdown.includes("temporary aggregate"), "Markdown copy preserves parallel_tasks payload and output");
ok(!markdown.includes("子任务失败") && !markdown.includes("正在执行"), "Markdown copy does not serialize localized presentation text");
ok(!markdown.includes("forceReveal"), "Markdown copy omits transient interruption reveal state");
ok(
  appSource.includes("renderSessionPdfBlob(getSessionMarkdown(), sessionTitle)")
    && appSource.includes("renderSessionImageBlob(getSessionMarkdown())"),
  "PDF and image exports share the filtered Markdown projection",
);

const json = JSON.parse(holder.json("runtime export", items)) as { items: typeof items };
ok(json.items[0]?.text === rawNotice, "JSON export preserves raw lifecycle Notice and task ID");
ok(json.items[0]?.id === "internal-notice-42", "JSON export preserves the internal item ID");
ok(!holder.json("runtime export", items).includes("forceReveal"), "JSON export omits transient interruption reveal state");
ok(json.items[1]?.text === rawPhase, "JSON export preserves raw Phase");
ok(json.items[2]?.name === "parallel_tasks" && json.items[2]?.output === "temporary aggregate", "JSON export preserves parallel_tasks data");

if (failed > 0) {
  process.stderr.write(`\n${failed} test(s) failed, ${passed} passed.\n`);
  process.exit(1);
}

process.stdout.write(`\n${passed} tests passed.\n`);
