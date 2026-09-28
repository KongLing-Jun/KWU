import assert from "node:assert/strict";

import { testdiagIsolationFindings } from "./testing-testdiag-guard.mjs";

assert.deepEqual(
  testdiagIsolationFindings(
    "kwu\nkwu/internal/agent\n",
    "kwu/internal/testdiag\n",
  ),
  [],
);

assert.deepEqual(
  testdiagIsolationFindings(
    "kwu\nkwu/internal/testdiag\n",
    "kwu/internal/testdiag\n",
  ),
  ["default package graph includes kwu/internal/testdiag"],
);

assert.deepEqual(
  testdiagIsolationFindings(
    "kwu\n",
    "",
  ),
  ["testdiag-tagged package graph excludes kwu/internal/testdiag"],
);

console.log("testdiag isolation guard tests: passed");
