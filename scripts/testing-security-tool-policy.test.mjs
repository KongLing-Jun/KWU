import assert from "node:assert/strict";

import {
  govulncheckVersion,
  securityToolPolicyFindings,
} from "./testing-security-tool-policy.mjs";

assert.equal(govulncheckVersion, "v1.6.0");
assert.deepEqual(
  securityToolPolicyFindings(
    "run: go install golang.org/x/vuln/cmd/govulncheck@v1.6.0",
  ),
  [],
);
assert.deepEqual(
  securityToolPolicyFindings(
    "run: go install golang.org/x/vuln/cmd/govulncheck@latest",
  ),
  [
    "govulncheck uses an unbounded @latest version",
    "govulncheck is not pinned to v1.6.0",
  ],
);

console.log("security tool policy tests: passed");
