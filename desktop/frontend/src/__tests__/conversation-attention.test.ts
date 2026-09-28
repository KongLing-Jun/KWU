// Run: tsx src/__tests__/conversation-attention.test.ts

import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import {
  conversationAttentionAccountScope,
  conversationAttentionIdentity,
  readConversationUnreadCompletions,
  shouldMarkTurnDoneUnread,
  withUnreadCompletion,
  withoutUnreadCompletion,
  writeConversationUnreadCompletions,
} from "../lib/conversationAttention";
import {
  highestProjectTreeAttentionStatus,
  projectTreeAggregateAttentionStatus,
  projectTreeAttentionStatus,
} from "../components/ProjectTree";
import type { ProjectNode } from "../lib/types";

let passed = 0;
let failed = 0;

function eq(actual: unknown, expected: unknown, label: string) {
  if (JSON.stringify(actual) === JSON.stringify(expected)) {
    process.stdout.write(`  PASS  ${label}\n`);
    passed += 1;
  } else {
    process.stdout.write(`  FAIL  ${label}: expected ${JSON.stringify(expected)}, got ${JSON.stringify(actual)}\n`);
    failed += 1;
  }
}

class MemoryStorage {
  private readonly values = new Map<string, string>();
  getItem(key: string) { return this.values.get(key) ?? null; }
  setItem(key: string, value: string) { this.values.set(key, value); }
}

console.log("\nconversation completion attention");

const accountA = conversationAttentionAccountScope("profile", "account-a");
const accountB = conversationAttentionAccountScope("profile", "account-b");
eq(
  conversationAttentionAccountScope(undefined, undefined),
  JSON.stringify(["profile", "local", "anonymous"]),
  "anonymous local sessions always have a persistent account scope",
);
const sessionA = conversationAttentionIdentity({ kind: "global_session", sessionPath: "C:\\sessions\\a.jsonl" });
const sessionB = conversationAttentionIdentity({ kind: "global_session", sessionPath: "C:\\sessions\\b.jsonl" });
const storage = new MemoryStorage();

writeConversationUnreadCompletions(accountA, withUnreadCompletion({}, sessionA, 1_700_000_000_000), storage);
eq(
  readConversationUnreadCompletions(accountA, storage),
  { [sessionA]: 1_700_000_000_000 },
  "completion survives a fresh storage read (application restart)",
);
eq(
  readConversationUnreadCompletions(accountB, storage),
  {},
  "different accounts cannot read each other's completion records",
);

const opened = withoutUnreadCompletion(readConversationUnreadCompletions(accountA, storage), sessionA);
writeConversationUnreadCompletions(accountA, opened, storage);
eq(readConversationUnreadCompletions(accountA, storage), {}, "opening a conversation clears only its unread completion");

eq(shouldMarkTurnDoneUnread(sessionB, sessionA), true, "successful background turn_done marks unread");
eq(shouldMarkTurnDoneUnread(sessionA, sessionA), false, "active turn_done stays read even when its runtime tab was detached");
eq(shouldMarkTurnDoneUnread(sessionA, ""), true, "completion on a non-conversation page stays unread");
eq(shouldMarkTurnDoneUnread(sessionB, sessionA, "failed"), false, "failed turn_done never marks completion unread");
eq(shouldMarkTurnDoneUnread(sessionB, sessionA, undefined, true), false, "interrupted turn_done never marks completion unread");

const unread = new Set([sessionB]);
const conversationNode = (status?: ProjectNode["status"], running = false): ProjectNode => ({
  key: "session-b",
  kind: "global_session",
  label: "B",
  sessionPath: "C:\\sessions\\b.jsonl",
  status,
  running,
});
eq(projectTreeAttentionStatus(conversationNode(), unread), "completed_unread", "idle unread conversation shows completion dot");
eq(projectTreeAttentionStatus(conversationNode("streaming"), unread), "running", "running outranks unread completion");
eq(projectTreeAttentionStatus(conversationNode("waiting_confirmation"), unread), "waiting_confirmation", "confirmation outranks running and unread");
eq(projectTreeAttentionStatus(conversationNode("error"), unread), "error", "failure has highest row priority");
eq(projectTreeAttentionStatus(conversationNode("paused"), unread), "paused", "paused remains neutral and is not remapped to completion");
eq(
  highestProjectTreeAttentionStatus("completed_unread", "running", "waiting_confirmation", "error"),
  "error",
  "collapsed conversation rows select the highest child state",
);

const collapsedProject: ProjectNode = {
  key: "project",
  kind: "project",
  label: "Project",
  root: "C:\\repo",
  children: [
    conversationNode(),
    { ...conversationNode("streaming"), key: "running", sessionPath: "C:\\sessions\\running.jsonl" },
    { ...conversationNode("waiting_confirmation"), key: "waiting", sessionPath: "C:\\sessions\\waiting.jsonl" },
    { ...conversationNode("error"), key: "error", sessionPath: "C:\\sessions\\error.jsonl" },
  ],
};
eq(projectTreeAggregateAttentionStatus(collapsedProject, unread), "error", "collapsed parent projects the highest child priority");
eq(
  projectTreeAggregateAttentionStatus({ ...collapsedProject, children: [conversationNode()] }, unread),
  "completed_unread",
  "collapsed parent preserves unread child completion without clearing it",
);
const collapsedTopic: ProjectNode = {
  key: "topic-with-sessions",
  kind: "global_topic",
  label: "Topic",
  topicId: "topic-with-sessions",
  children: [
    { ...conversationNode(), key: "idle-child", sessionPath: "C:\\sessions\\idle.jsonl" },
    { ...conversationNode("streaming"), key: "running-child", sessionPath: "C:\\sessions\\running.jsonl" },
  ],
};
eq(
  highestProjectTreeAttentionStatus(
    projectTreeAttentionStatus(collapsedTopic, unread),
    projectTreeAggregateAttentionStatus(collapsedTopic, unread),
  ),
  "running",
  "a collapsed topic projects its hidden running session onto the visible row",
);

const testDir = dirname(fileURLToPath(import.meta.url));
const baseSource = readFileSync(resolve(testDir, "../styles/base.css"), "utf8");
const themeSource = readFileSync(resolve(testDir, "../styles/theme-overrides.css"), "utf8");
const attentionStyleSource = `${baseSource}\n${themeSource}`;
const treeSource = readFileSync(resolve(testDir, "../features/workspace/ui/ProjectTree.tsx"), "utf8");
const appSource = readFileSync(resolve(testDir, "../App.tsx"), "utf8");
const bridgeSource = readFileSync(resolve(testDir, "../lib/bridge.ts"), "utf8");
eq(
  /project-tree__topic:hover \.project-tree__topic-time[\s\S]*?opacity:\s*0/.test(themeSource)
    && !/project-tree__topic:hover \.project-tree__topic-side[\s\S]{0,400}?opacity:\s*0/.test(themeSource),
  true,
  "Creation hover replaces the time without hiding the critical state slot",
);
const creationActionsRight = themeSource.match(
  /\.app--creation \.project-tree__topic-actions,[\s\S]*?right:\s*(\d+)px/,
)?.[1];
eq(
  Number(creationActionsRight) >= 36
    && /project-tree__topic--has-actions \.project-tree__topic-side[\s\S]*?min-width:\s*76px/.test(themeSource),
  true,
  "Creation hover actions reserve a separate lane before the persistent state marker",
);
eq(
  !/project-tree__topic--attention-running[^\{]*\.project-tree__topic-actions\s*\{[^}]*right:/.test(attentionStyleSource)
    && !/project-tree__topic--attention-running[^\{]*\.project-tree__state-slot\s*\{[^}]*opacity:\s*0/.test(attentionStyleSource),
  true,
  "Running indicators stay visible during hover, focus and menu interactions without moving row actions",
);
eq(
  /!isExpanded[\s\S]*?projectTreeAggregateAttentionStatus\(node, unreadCompletionIds\)/.test(treeSource),
  true,
  "parent aggregation renders only while the project is collapsed",
);
eq(
  bridgeSource.includes('const CONVERSATION_ACTIVITY_EVENT_CHANNEL = "conversation:activity"')
    && appSource.includes("const unsub = onConversationActivity((e) =>"),
  true,
  "completion attention subscribes to the app-scoped lifecycle channel",
);
eq(
  /completedSuccessfully[\s\S]*?updateConversationUnreadCompletion\(eventAccountScope, turnConversationIdentity, Date\.now\(\)\);[\s\S]*?window\.setTimeout/.test(appSource),
  true,
  "background completion is persisted before tab metadata refresh",
);

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
