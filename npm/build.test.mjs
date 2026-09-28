import assert from "node:assert/strict";
import { readFileSync } from "node:fs";

const source = readFileSync(new URL("./build.mjs", import.meta.url), "utf8");

assert.match(source, /const versionStamp = `v\$\{version\}`;/, "CLI binary version stamp should use normalized package version");
assert.doesNotMatch(source, /main\.version=\$\{tag\}/, "CLI binary version stamp must not use the raw npm tag");

console.log("npm build contract passed");
