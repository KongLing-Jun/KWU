import assert from "node:assert/strict";

import { classifyFiles } from "./pr-gate.mjs";

const docs = classifyFiles(["docs/GUIDE.zh-CN.md"]);
assert.equal(docs.rootGo.mode, "none");
assert.equal(docs.desktopGo.mode, "none");
assert.equal(docs.frontend.mode, "none");

const markdownGate = classifyFiles(["scripts/check-markdown-links.mjs"]);
assert.equal(markdownGate.gateContract, true);

const leafGo = classifyFiles(["internal/bot/feishu/feishu.go"]);
assert.equal(leafGo.rootGo.mode, "targeted");
assert.deepEqual(leafGo.rootGo.packages, ["./internal/bot/feishu"]);

const coreGo = classifyFiles(["internal/control/controller.go"]);
assert.equal(coreGo.rootGo.mode, "targeted");
assert.deepEqual(coreGo.rootGo.packages, ["./internal/control"]);

const boundedCoreGo = classifyFiles(["internal/control/task_lifecycle.go"]);
assert.equal(boundedCoreGo.rootGo.mode, "targeted");
assert.deepEqual(boundedCoreGo.rootGo.packages, ["./internal/control"]);

const dependency = classifyFiles(["go.mod"]);
assert.equal(dependency.rootGo.mode, "full");

const mediumGo = classifyFiles(
  Array.from({ length: 9 }, (_, index) => `internal/leaf${index}/leaf.go`),
);
assert.equal(mediumGo.rootGo.mode, "targeted");

const broadGo = classifyFiles(
  Array.from({ length: 13 }, (_, index) => `internal/leaf${index}/leaf.go`),
);
assert.equal(broadGo.rootGo.mode, "full");

const broadCoreGo = classifyFiles([
  "internal/agent/agent.go",
  "internal/billing/billing.go",
  "internal/boot/boot.go",
  "internal/config/config.go",
  "internal/control/controller.go",
]);
assert.equal(broadCoreGo.rootGo.mode, "full");

const desktopLeaf = classifyFiles(["desktop/cmd/sign/main.go"]);
assert.equal(desktopLeaf.desktopGo.mode, "targeted");
assert.deepEqual(desktopLeaf.desktopGo.packages, ["./cmd/sign"]);

const desktopApp = classifyFiles(["desktop/app.go"]);
assert.equal(desktopApp.desktopGo.mode, "targeted");
assert.deepEqual(desktopApp.desktopGo.packages, ["."]);

const desktopDependency = classifyFiles(["desktop/go.mod"]);
assert.equal(desktopDependency.desktopGo.mode, "full");

const todo = classifyFiles([
  "desktop/frontend/src/components/TodoPanel.tsx",
  "desktop/frontend/src/lib/todoVisibility.ts",
]);
assert.equal(todo.frontend.mode, "build");
assert.deepEqual(todo.frontend.tests, []);
assert.equal(todo.frontend.build, true);
assert.equal(todo.rootGo.mode, "none");
assert.equal(todo.desktopGo.mode, "none");

const unknownFrontend = classifyFiles(["desktop/frontend/src/components/NewSurface.tsx"]);
assert.equal(unknownFrontend.frontend.mode, "build");
assert.equal(unknownFrontend.frontend.build, true);

const sharedFrontend = classifyFiles(["desktop/frontend/src/locales/zh.ts"]);
assert.equal(sharedFrontend.frontend.mode, "build");
assert.equal(sharedFrontend.frontend.build, true);

const frontendApp = classifyFiles(["desktop/frontend/src/App.tsx"]);
assert.equal(frontendApp.frontend.mode, "build");
assert.equal(frontendApp.frontend.build, true);

const frontendInfrastructure = classifyFiles(["desktop/frontend/package.json"]);
assert.equal(frontendInfrastructure.frontend.mode, "build");
assert.equal(frontendInfrastructure.frontend.build, true);

const styles = classifyFiles(["desktop/frontend/src/styles/settings.css"]);
assert.equal(styles.frontend.mode, "build");
assert.equal(styles.frontend.build, true);

const packaging = classifyFiles(["desktop/build/windows/info.json"]);
assert.equal(packaging.releaseContract, true);

for (const path of [
  "scripts/pr-gate.mjs",
  "scripts/testing-inventory.mjs",
  "scripts/testing-phase-gates.mjs",
  "scripts/testing-baseline.ps1",
  "scripts/testing-baseline-summary.mjs",
  "scripts/testing-frontend-license-policy.mjs",
  "scripts/testing-webview2-cdp.ps1",
  "scripts/testing-webview2-cdp-build.ps1",
  "scripts/testing-windows-native-smoke.ps1",
  "scripts/testing-windows-uia-tree.ps1",
  "scripts/testing-windows-uia-report.mjs",
  "scripts/testing-process-soak.ps1",
  "scripts/testing-process-soak-test.ps1",
  "scripts/testing-release-evidence-policy.mjs",
  "scripts/testing-security-tool-policy.mjs",
  "scripts/testing-testdiag-guard.mjs",
]) {
  const gate = classifyFiles([path]);
  assert.equal(gate.gateContract, true, path);
  assert.equal(gate.rootGo.mode, "none", path);
  assert.equal(gate.desktopGo.mode, "none", path);
  assert.equal(gate.frontend.mode, "none", path);
}

console.log("pr-gate classification: all checks passed");
