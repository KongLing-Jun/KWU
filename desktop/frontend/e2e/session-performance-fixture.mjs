import { createWriteStream } from "node:fs";
import { mkdir, writeFile, utimes } from "node:fs/promises";
import path from "node:path";
import { once } from "node:events";

const profileIndex = process.argv.indexOf("--profile-root");
if (profileIndex < 0 || !process.argv[profileIndex + 1]) {
  console.error("usage: node e2e/session-performance-fixture.mjs --profile-root <absolute-path>");
  process.exit(2);
}

const profileRoot = path.resolve(process.argv[profileIndex + 1]);
if (!path.isAbsolute(profileRoot)) {
  throw new Error("profile root must be absolute");
}

const sessionsDir = path.join(profileRoot, "sessions");
const subagentsDir = path.join(sessionsDir, "subagents");
const longSessionID = "perf-long-session";
const longSessionPath = path.join(sessionsDir, `${longSessionID}.jsonl`);
const longSessionMetaPath = `${longSessionPath}.meta`;
const subagentRef = "sa_perf";
const subagentPath = path.join(subagentsDir, `${subagentRef}.jsonl`);
const now = new Date();

await mkdir(subagentsDir, { recursive: true });

function line(message) {
  return `${JSON.stringify(message)}\n`;
}

async function writeLines(filePath, messages) {
  const stream = createWriteStream(filePath, { encoding: "utf8" });
  for (const message of messages) {
    if (!stream.write(line(message))) {
      await once(stream, "drain");
    }
  }
  stream.end();
  await once(stream, "finish");
}

async function writeMeta(sessionPath, meta) {
  await writeFile(`${sessionPath}.meta`, `${JSON.stringify(meta, null, 2)}\n`, "utf8");
}

const smallCreated = new Date(now.getTime() - 86_400_000);
for (let i = 0; i < 499; i += 1) {
  const id = `perf-session-${String(i).padStart(3, "0")}`;
  const sessionPath = path.join(sessionsDir, `${id}.jsonl`);
  const messages = i === 0
    ? [
        { role: "system", content: "Deterministic subagent performance fixture." },
        { role: "user", content: "PERF-SUBAGENT-SESSION" },
        {
          role: "assistant",
          content: "",
          tool_calls: [{ id: "perf_task_call", name: "task", arguments: "{\"description\":\"fixture\"}" }],
        },
        { role: "tool", content: "Subagent fixture completed.", tool_call_id: "perf_task_call", name: "task" },
        { role: "assistant", content: "PERF-SUBAGENT-PARENT-LAST" },
      ]
    : [
        { role: "system", content: "Deterministic performance fixture." },
        { role: "user", content: `PERF-SMALL-${i}` },
        { role: "assistant", content: `Fixture response ${i}` },
      ];
  const preview = i === 0 ? "PERF-SUBAGENT-SESSION" : `PERF-SMALL-${i}`;
  await writeLines(sessionPath, messages);
  await writeMeta(sessionPath, {
    id,
    name: preview,
    created_at: smallCreated.toISOString(),
    updated_at: new Date(smallCreated.getTime() + i * 1000).toISOString(),
    scope: "global",
    schema_version: 1,
    turns: messages.length,
    preview,
  });
}

const largePayload = "x".repeat(10_000);
const largePairs = 10_500;
const longStream = createWriteStream(longSessionPath, { encoding: "utf8" });
const writeLong = async (message) => {
  if (!longStream.write(line(message))) {
    await once(longStream, "drain");
  }
};
await writeLong({ role: "system", content: "Deterministic 100MB history fixture." });
for (let i = 0; i < largePairs; i += 1) {
  await writeLong({ role: "user", content: `PERF-LONG-USER-${i}` });
  await writeLong({ role: "assistant", content: `${largePayload}-${i}` });
}
await writeLong({ role: "user", content: "PERF-TASK-USER" });
await writeLong({ role: "assistant", content: "PERF-LONG-LAST" });
longStream.end();
await once(longStream, "finish");

const longMessages = 1 + largePairs * 2 + 2;
await writeFile(longSessionMetaPath, `${JSON.stringify({
  id: longSessionID,
  name: "PERF-LONG-SESSION",
  created_at: new Date(now.getTime() - 3_600_000).toISOString(),
  updated_at: now.toISOString(),
  scope: "global",
  schema_version: 1,
  turns: longMessages,
  preview: "PERF-LONG-SESSION",
}, null, 2)}\n`, "utf8");

const subagentMessages = [];
for (let i = 0; i < 799; i += 1) {
  subagentMessages.push({
    role: i % 2 === 0 ? "user" : "assistant",
    content: `PERF-SUBAGENT-${i}`,
  });
}
subagentMessages.push({ role: "assistant", content: "PERF-SUBAGENT-LAST" });
await writeLines(subagentPath, subagentMessages);
await writeFile(path.join(subagentsDir, `${subagentRef}.meta.json`), `${JSON.stringify({
  ref: subagentRef,
  createdAt: new Date(now.getTime() - 1_800_000).toISOString(),
  updatedAt: now.toISOString(),
  status: "completed",
  kind: "task",
  name: "fixture",
  workspaceRoot: "",
  parentSession: "perf-session-000",
  parentToolCallId: "perf_task_call",
  systemPromptHash: "",
  toolScope: [],
  toolSchemaHash: "",
  model: "fixture",
  effort: "low",
}, null, 2)}\n`, "utf8");

for (const target of [longSessionPath, longSessionMetaPath, subagentPath]) {
  await utimes(target, now, now);
}

const longBytes = (await import("node:fs/promises")).stat(longSessionPath).then((stat) => stat.size);
const subagentBytes = (await import("node:fs/promises")).stat(subagentPath).then((stat) => stat.size);
process.stdout.write(`${JSON.stringify({
  schemaVersion: 1,
  sessionCount: 500,
  longSession: {
    id: longSessionID,
    path: longSessionPath,
    messages: longMessages,
    bytes: await longBytes,
  },
  subagent: {
    ref: subagentRef,
    path: subagentPath,
    messages: subagentMessages.length,
    bytes: await subagentBytes,
  },
})}\n`);
