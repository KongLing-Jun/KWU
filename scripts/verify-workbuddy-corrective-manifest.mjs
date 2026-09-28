import { createHash } from "node:crypto";
import { execFileSync } from "node:child_process";
import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";

function fail(message) {
  throw new Error(`workbuddy corrective manifest: ${message}`);
}

function parseArgs(argv) {
  const values = {};
  for (let index = 0; index < argv.length; index += 2) {
    const key = argv[index];
    const value = argv[index + 1];
    if (!key?.startsWith("--") || !value) fail("expected --name value arguments");
    values[key.slice(2)] = resolve(value);
  }
  for (const key of ["manifest", "desktop", "cloud", "admin"]) {
    if (!values[key]) fail(`missing --${key}`);
  }
  return values;
}

function sha256(path) {
  return createHash("sha256").update(readFileSync(path)).digest("hex");
}

function parsedJSONHash(path) {
  return createHash("sha256")
    .update(JSON.stringify(JSON.parse(readFileSync(path, "utf8"))))
    .digest("hex");
}

const paths = parseArgs(process.argv.slice(2));
const manifest = JSON.parse(readFileSync(paths.manifest, "utf8"));
if (manifest.schema_version !== 1) fail("unsupported schema_version");

for (const name of ["desktop", "cloud", "admin"]) {
  const pinned = manifest.repositories?.[name]?.implementation_commit;
  if (!/^[0-9a-f]{40}$/.test(pinned ?? "")) fail(`${name} implementation commit is invalid`);
  try {
    execFileSync("git", ["-C", paths[name], "merge-base", "--is-ancestor", pinned, "HEAD"], {
      stdio: "ignore",
    });
  } catch {
    fail(`${name} implementation commit is not an ancestor of HEAD`);
  }
}

const cloudContract = resolve(paths.cloud, manifest.contract.cloud_path);
const adminContract = resolve(paths.admin, manifest.contract.admin_path);
if (
  parsedJSONHash(cloudContract) !== manifest.contract.parsed_sha256 ||
  parsedJSONHash(adminContract) !== manifest.contract.parsed_sha256
) {
  fail("Cloud/Admin parsed contract mismatch");
}

for (const test of manifest.required_tests ?? []) {
  if (!test.name || test.status !== "PASS" || test.discovered < 1 || test.failed !== 0 || test.skipped !== 0) {
    fail(`required test is missing, failed or skipped: ${test.name ?? "unnamed"}`);
  }
}

const images = manifest.images ?? [];
if (images.filter((item) => item.repository === "desktop").length !== 6) fail("six Desktop image rows are required");
if (images.filter((item) => item.repository === "admin").length !== 2) fail("two Admin live image rows are required");
for (const item of images) {
  const root = paths[item.repository];
  const file = root && resolve(root, item.path);
  if (!root || item.result !== "PASS" || !file || !existsSync(file) || sha256(file) !== item.sha256) {
    fail(`image row is missing, failed or stale: ${item.path ?? "unnamed"}`);
  }
}

if (!manifest.live_acceptance?.desktop_real_cloud_postgresql || !manifest.live_acceptance?.admin_real_cloud_postgresql) {
  fail("real Desktop/Admin live acceptance is incomplete");
}
for (const severity of ["critical", "warning", "suggestion"]) {
  if (manifest.final_review?.[severity] !== 0) fail(`final review ${severity} findings remain`);
}

console.log(
  JSON.stringify({
    status: "PASS",
    repositories: 3,
    required_tests: manifest.required_tests.length,
    desktop_images: 6,
    admin_images: 2,
    parsed_contract_sha256: manifest.contract.parsed_sha256,
  }),
);
