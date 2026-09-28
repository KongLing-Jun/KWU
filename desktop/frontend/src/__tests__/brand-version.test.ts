import { strict as assert } from "node:assert";
import { formatBrandVersion } from "../lib/brand";

assert.equal(formatBrandVersion("5.6.0"), "v5.6.0");
assert.equal(formatBrandVersion("v5.6.0"), "v5.6.0");
assert.equal(formatBrandVersion(" v5.6.0 "), "v5.6.0");
assert.equal(formatBrandVersion("V5.6.0"), "v5.6.0");
assert.equal(formatBrandVersion(" v "), "");
assert.equal(formatBrandVersion("  "), "");
