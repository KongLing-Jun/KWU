// Run: tsx src/__tests__/composer-workspace-picker.test.ts

import { shouldShowComposerWorkspacePicker } from "../lib/composerWorkspacePicker";

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

console.log("\ncomposer workspace picker");

ok(
  shouldShowComposerWorkspacePicker({
    workspaceView: "chat",
    sidebarImDetailConnection: false,
    activeTabId: "tab-a",
    activeTabScope: "global",
    activeTabReadOnly: false,
    currentSessionTurns: 0,
    activeTopicTurns: 3,
  }),
  "blank global session shows workspace picker even when topic has older turns",
);

ok(
  !shouldShowComposerWorkspacePicker({
    workspaceView: "chat",
    sidebarImDetailConnection: false,
    activeTabId: "tab-a",
    activeTabScope: "global",
    activeTabReadOnly: false,
    currentSessionTurns: 1,
    activeTopicTurns: 1,
  }),
  "existing session hides workspace picker",
);

ok(
  !shouldShowComposerWorkspacePicker({
    workspaceView: "chat",
    sidebarImDetailConnection: false,
    activeTabId: "tab-a",
    activeTabScope: "project",
    activeTabReadOnly: false,
    currentSessionTurns: 0,
  }),
  "project session hides workspace picker because workspace is already visible in sidebar",
);

if (failed > 0) process.exit(1);
