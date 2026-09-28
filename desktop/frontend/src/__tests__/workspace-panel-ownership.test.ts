// Run: tsx src/__tests__/workspace-panel-ownership.test.ts

import { DEFAULT_WORKSPACE_PANEL_STATE, useLayoutStore } from "../store/layout";

let passed = 0;
let failed = 0;

function eq(actual: unknown, expected: unknown, label: string) {
  if (actual === expected) {
    process.stdout.write(`  PASS  ${label}\n`);
    passed += 1;
  } else {
    process.stdout.write(`  FAIL  ${label}: expected ${JSON.stringify(expected)}, got ${JSON.stringify(actual)}\n`);
    failed += 1;
  }
}

console.log("\nworkspace panel ownership");

const store = useLayoutStore.getState();
eq(store.workspacePanelsByConversation["chat-1"], undefined, "chat 1 starts without owned dock state");
eq(store.workspacePanelsByConversation["chat-2"], undefined, "chat 2 starts without owned dock state");

store.setWorkspacePanelState("chat-1", (current) => ({
  ...current,
  open: true,
  mode: "changed",
}));

eq(useLayoutStore.getState().workspacePanelsByConversation["chat-1"]?.open, true, "chat 1 remembers its open dock");
eq(useLayoutStore.getState().workspacePanelsByConversation["chat-1"]?.mode, "changed", "chat 1 remembers its dock mode");
eq(
  useLayoutStore.getState().workspacePanelsByConversation["chat-2"]?.open ?? DEFAULT_WORKSPACE_PANEL_STATE.open,
  false,
  "chat 2 remains closed when chat 1 opens the dock",
);

store.setWorkspacePanelState("chat-2", (current) => ({
  ...current,
  open: true,
  mode: "files",
}));

eq(useLayoutStore.getState().workspacePanelsByConversation["chat-1"]?.mode, "changed", "chat 2 changes do not overwrite chat 1");
eq(useLayoutStore.getState().workspacePanelsByConversation["chat-2"]?.mode, "files", "chat 2 keeps its own dock mode");

console.log(`\n${passed} passed, ${failed} failed, ${passed + failed} total`);
if (failed > 0) process.exit(1);
