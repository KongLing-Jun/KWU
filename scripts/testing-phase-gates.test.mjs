import assert from "node:assert/strict";

import {
  commandInvocation,
  gateManifest,
  PHASE_GATES,
  QUALIFIED_GATES,
} from "./testing-phase-gates.mjs";

const manifest = gateManifest();
assert.equal(manifest.schemaVersion, 1);
assert.deepEqual(
  [...new Set(PHASE_GATES.map((gate) => gate.phase))].sort(),
  ["phase1", "phase2", "phase6"],
);
assert.equal(new Set(PHASE_GATES.map((gate) => gate.id)).size, PHASE_GATES.length);
assert.ok(PHASE_GATES.every((gate) => gate.command && gate.args.length > 0));
assert.ok(
  PHASE_GATES.some(
    (gate) =>
      gate.phase === "phase1" &&
      gate.args.includes("./internal/cloudregistration") &&
      gate.args.includes("./internal/installsource"),
  ),
);
assert.ok(
  PHASE_GATES.some(
    (gate) =>
      gate.phase === "phase1" &&
      gate.workingDirectory === "desktop" &&
      gate.args.includes("./internal/update"),
  ),
);
assert.ok(
  PHASE_GATES.some(
    (gate) =>
      gate.phase === "phase2" &&
      gate.args.includes("scripts/testing-webview2-production-guard.mjs"),
  ),
);
assert.ok(
  PHASE_GATES.some(
    (gate) =>
      gate.phase === "phase6" &&
      gate.args.includes("scripts/pr-gate.test.mjs"),
  ),
);
assert.deepEqual(
  QUALIFIED_GATES.map((gate) => gate.id),
  ["live-webview2-cdp", "live-windows-uia", "final-installer-diagnostic-isolation"],
);
assert.ok(QUALIFIED_GATES.every((gate) => gate.requiredInputs.length > 0 && gate.command));

const windowsCmd = commandInvocation(
  { command: "pnpm.cmd", args: ["--dir", "desktop/frontend", "test:vitest"] },
  "win32",
  "C:\\Windows\\System32\\cmd.exe",
);
assert.equal(windowsCmd.command, "C:\\Windows\\System32\\cmd.exe");
assert.deepEqual(windowsCmd.args.slice(0, 3), ["/d", "/s", "/c"]);
assert.equal(
  windowsCmd.args[3],
  "pnpm.cmd --dir desktop/frontend test:vitest",
);

console.log("testing phase gates: all checks passed");
