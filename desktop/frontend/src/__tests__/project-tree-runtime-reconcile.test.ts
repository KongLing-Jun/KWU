// Run: tsx src/__tests__/project-tree-runtime-reconcile.test.ts

import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { projectTreeRuntimeKeyForTabs } from "../features/session/sessionMeta";
import type { TabMeta } from "../lib/types";

let passed = 0;
let failed = 0;
const testDir = dirname(fileURLToPath(import.meta.url));
const appSource = readFileSync(resolve(testDir, "../App.tsx"), "utf8");

function eq(a: unknown, b: unknown, label: string) {
  if (JSON.stringify(a) === JSON.stringify(b)) {
    process.stdout.write(`  PASS  ${label}\n`);
    passed += 1;
  } else {
    process.stdout.write(`  FAIL  ${label}: expected ${JSON.stringify(b)}, got ${JSON.stringify(a)}\n`);
    failed += 1;
  }
}

function tab(overrides: Partial<TabMeta> = {}): TabMeta {
  return {
    id: "tab-1",
    scope: "global",
    workspaceRoot: "",
    workspaceName: "Global",
    topicId: "topic-1",
    topicTitle: "Topic",
    label: "Task",
    ready: true,
    running: false,
    mode: "normal",
    active: true,
    cwd: "",
    ...overrides,
  };
}

function runtimeRevisions(snapshots: TabMeta[][]): number[] {
  let baseline: string | null = null;
  let hasBaseline = false;
  let revision = 0;
  return snapshots.map((tabs) => {
    const key = projectTreeRuntimeKeyForTabs(tabs);
    if (!hasBaseline) {
      if (tabs.length === 0) return revision;
      hasBaseline = true;
      baseline = key;
      return revision;
    }
    if (baseline === key) return revision;
    baseline = key;
    revision += 1;
    return revision;
  });
}

console.log("\nproject tree runtime reconciliation");

const runningTab = tab({ id: "active", sessionPath: "/sessions/active.jsonl", running: true });
const backgroundTab = tab({ id: "background", sessionPath: "/sessions/background.jsonl", active: false, running: true });

eq(
  runtimeRevisions([[], [runningTab], [tab({ ...runningTab, running: false })], [tab({ ...runningTab, running: false })]]),
  [0, 0, 1, 1],
  "first metadata load establishes a baseline and running to idle increments the tree revision once",
);

eq(
  runtimeRevisions([
    [runningTab, backgroundTab],
    [runningTab, tab({ ...backgroundTab, running: false })],
  ]),
  [0, 1],
  "a background tab runtime transition increments the shared tree revision",
);

eq(
  projectTreeRuntimeKeyForTabs([tab({ pendingPrompt: true })]) !== projectTreeRuntimeKeyForTabs([tab({ pendingPrompt: false })]),
  true,
  "pending prompt changes are part of the runtime key",
);

eq(
  projectTreeRuntimeKeyForTabs([tab({ backgroundJobs: 1 })]) !== projectTreeRuntimeKeyForTabs([tab({ backgroundJobs: 0 })]),
  true,
  "background job changes are part of the runtime key",
);

eq(
  projectTreeRuntimeKeyForTabs([tab({ sessionPath: "/sessions/one.jsonl" })]) !== projectTreeRuntimeKeyForTabs([tab({ sessionPath: "/sessions/two.jsonl" })]),
  true,
  "session rebind changes are part of the runtime key",
);

eq(
  projectTreeRuntimeKeyForTabs([tab({ active: true, label: "old", modelRef: "model-a", effort: "low", tokenMode: "full", goal: "old" })]),
  projectTreeRuntimeKeyForTabs([tab({ active: false, label: "new", modelRef: "model-b", effort: "high", tokenMode: "economy", goal: "new" })]),
  "active state and ordinary metadata do not change the runtime key",
);

eq(
  runtimeRevisions([
    [runningTab],
    [tab({ ...runningTab, label: "renamed", modelRef: "model-b", effort: "high", goal: "updated" })],
    [tab({ ...runningTab, label: "renamed", modelRef: "model-b", effort: "high", goal: "updated" })],
  ]),
  [0, 0, 0],
  "unchanged runtime snapshots and unrelated metadata remain a revision no-op",
);

eq(
  runtimeRevisions([
    [tab({ ...runningTab, running: true, backgroundJobs: 2 })],
    [tab({ ...runningTab, running: true, backgroundJobs: 2, label: "Todo complete" })],
    [tab({ ...runningTab, running: true, backgroundJobs: 0, label: "Todo complete" })],
  ]),
  [0, 0, 1],
  "background activity remains observable even when unrelated completion metadata changes",
);

const runtimeRefreshBlock = appSource.match(
  /const projectTreeRuntimeKey = useMemo\(\(\) => projectTreeRuntimeKeyForTabs\(tabMetas\), \[tabMetas\]\);[\s\S]*?const lastSeenProjectTreeRevisionRef/,
)?.[0] ?? "";
eq(
  appSource.includes("projectTreeRuntimeKeyForTabs") && runtimeRefreshBlock.includes("projectTreeRuntimeKeyForTabs(tabMetas)"),
  true,
  "App computes the production project-tree runtime key from tab metadata",
);
eq(
  /if \(!hasProjectTreeRuntimeBaselineRef\.current\) \{[\s\S]*?if \(tabMetas\.length === 0\) return;[\s\S]*?projectTreeRuntimeKeyRef\.current = projectTreeRuntimeKey;[\s\S]*?return;/.test(runtimeRefreshBlock),
  true,
  "App establishes the first non-empty metadata snapshot without an extra tree refresh",
);
eq(
  /if \(projectTreeRuntimeKeyRef\.current === projectTreeRuntimeKey\) return;[\s\S]*?setProjectRevision\(\(value\) => value \+ 1\);/.test(runtimeRefreshBlock),
  true,
  "App increments the existing project revision only after a later runtime-key change",
);
eq(
  runtimeRefreshBlock.includes("[projectTreeRuntimeKey, tabMetas.length]"),
  true,
  "App keeps the reconciliation effect tied to the metadata runtime snapshot",
);

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
