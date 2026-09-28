import assert from "node:assert/strict";

import {
  allowedLicenses,
  classifyLicenseGroups,
} from "./testing-frontend-license-policy.mjs";

assert.deepEqual([...allowedLicenses].sort(), [
  "BSD-2-Clause",
  "BSD-3-Clause",
  "ISC",
  "MIT",
]);

assert.deepEqual(
  classifyLicenseGroups({
    MIT: [{ name: "permissive" }],
    "SEE LICENSE IN LICENSE.txt": [{ name: "custom" }],
    "AGPL-3.0-only": [{ name: "copyleft" }],
  }),
  [
    {
      license: "AGPL-3.0-only",
      decision: "deny",
      packages: ["copyleft"],
    },
    {
      license: "MIT",
      decision: "allow",
      packages: ["permissive"],
    },
    {
      license: "SEE LICENSE IN LICENSE.txt",
      decision: "review",
      packages: ["custom"],
    },
  ],
);

console.log("frontend license policy tests: passed");
